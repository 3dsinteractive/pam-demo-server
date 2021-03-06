package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/parnurzeal/gorequest"
	"github.com/segmentio/fasthash/fnv1a"
)

type user struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Mobile   string `json:"mobile"`
	Password string `json:"-"`
	Database string `json:"database"`
}

type registerPostBody struct {
	Email      string `json:"email"`
	Mobile     string `json:"mobile"`
	Password   string `json:"password"`
	ConsentIDs string `json:"consent_ids"` // comma separated value of consent allow ids
}

type loginPostBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

var e *echo.Echo
var users map[string]*user
var req *gorequest.SuperAgent

func main() {
	initVariables()
	registerServices()

	// Start services
	e.Start(":8080")
}

func initVariables() {
	e = echo.New()
	users = readUsersFromEnv()
	req = gorequest.New()
}

func registerServices() error {

	// e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
	// 	Skipper:          middleware.DefaultSkipper,
	// 	AllowOrigins:     []string{"*"},
	// 	AllowHeaders:     []string{},
	// 	AllowMethods:     []string{echo.GET, echo.PUT, echo.POST, echo.DELETE, echo.HEAD, echo.OPTIONS, echo.PATCH},
	// 	AllowCredentials: true,
	// }))

	e.Use(middleware.CORS())

	e.GET("/", handleDefault)
	e.POST("/login", handleLogin)
	e.POST("/register", handleRegister)
	return nil
}

func handleDefault(ctx echo.Context) error {
	responseSuccess(ctx, map[string]string{"status": "ok"})
	return nil
}

func handleRegister(ctx echo.Context) error {
	postBody, err := readRequestBody(ctx)
	if err != nil {
		return err
	}

	registerBody := &registerPostBody{}
	err = json.Unmarshal([]byte(postBody), &registerBody)
	if err != nil {
		return err
	}

	user := findUser(registerBody.Email)
	if user == nil {
		responseError(ctx, "This email is not allowed to register with this api")
		return nil
	}

	endpoint := readPAMEndpointFromEnv()
	authToken := readPAMAuthTokenFromEnv()
	database := readDatabaseFromEnv()
	hashedCustID := hashCustomerID(user.UserID)

	_, body, err := postRegisterEventToPAMTracker(
		endpoint, authToken, database, hashedCustID,
		registerBody.ConsentIDs, user.Email, registerBody.Mobile)

	logMessage(body)

	res := map[string]string{
		"customer_id": hashedCustID,
		"email":       user.Email,
	}
	responseSuccess(ctx, res)

	return nil
}

func hashCustomerID(customerID string) string {
	hashed := fnv1a.HashString64(customerID)
	return fmt.Sprintf("%d", hashed)
}

func handleLogin(ctx echo.Context) error {
	postBody, err := readRequestBody(ctx)
	if err != nil {
		return err
	}

	loginBody := &loginPostBody{}
	err = json.Unmarshal([]byte(postBody), &loginBody)
	if err != nil {
		return err
	}

	user := findUser(loginBody.Email)
	if user == nil {
		responseError(ctx, "User not found")
		return nil
	}

	hashedCustID := hashCustomerID(user.UserID)
	res := map[string]string{
		"customer_id": hashedCustID,
	}

	responseSuccess(ctx, res)

	return nil
}

func readPAMEndpointFromEnv() string {
	endpoint := os.Getenv("PAM_ENDPOINT")
	if len(endpoint) == 0 {
		// Default endpoint
		endpoint = "https://stgx.pams.ai"
	}
	return endpoint
}

func logMessage(message string) {
	fmt.Println(message)
}

func responseError(ctx echo.Context, message string) {
	res := map[string]interface{}{
		"error": message,
	}
	responseJSON(ctx, res)
}

func responseSuccess(ctx echo.Context, response interface{}) {
	res := map[string]interface{}{
		"data": response,
	}
	responseJSON(ctx, res)
}

func responseJSON(ctx echo.Context, response interface{}) error {
	str, err := json.Marshal(response)
	if err != nil {
		return err
	}
	writer := ctx.Response().Writer
	writer.Header().Set("Content-Type", "application/json")
	writer.Write(str)

	return nil
}

func findUser(email string) *user {
	user, ok := users[email]
	if ok {
		return user
	}
	return nil
}

func postRegisterEventToPAMTracker(
	endpoint string,
	authToken string,
	database string,
	customerID string,
	consentAllowID string,
	email string,
	mobile string) (*http.Response, string, error) {

	postBody := map[string]interface{}{
		"event": "register",
		"form_fields": map[string]string{
			"_database":    database,
			"_consent_ids": consentAllowID,
			"customer":     customerID,
			"email":        email,
		},
	}

	if len(mobile) > 0 {
		formFields := postBody["form_fields"].(map[string]string)
		formFields["sms"] = mobile
		postBody["form_fields"] = formFields
	}

	url := fmt.Sprint(endpoint, "/trackers/events")
	req := getRequester()
	req = req.Post(url)

	req.Cookies = []*http.Cookie{}
	req.Header.Set("Authorization", authToken)
	req.Header.Set("x-allow-contact-id-cookie", "false")
	logMessage(fmt.Sprintf("Authorization=%s", authToken))

	// Log post body
	postBodyJS, err := json.Marshal(postBody)
	if err != nil {
		return nil, "", err
	}
	logMessage(string(postBodyJS))

	req.Send(postBody)

	res, body, errs := req.End()
	if len(errs) > 0 {
		if res != nil {
			return res, body, errs[0]
		}
		return res, body, errs[0]
	}
	return res, body, nil
}

func getRequester() *gorequest.SuperAgent {
	// Timeout is relative to time.Now so we need to set every time
	req.Timeout(30 * time.Second)
	return req.Clone()
}

func readRequestBody(ctx echo.Context) (string, error) {
	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func readUsersFromEnv() map[string]*user {
	userJS := os.Getenv("USERS")
	usersArrs := []*user{}
	err := json.Unmarshal([]byte(userJS), &usersArrs)
	if err != nil {
		usersArrs = getDefaultUsers()
	}
	users := map[string]*user{}
	for _, usr := range usersArrs {
		users[usr.Email] = usr
	}
	return users
}

func getDefaultUsers() []*user {
	userNames := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}
	users := []*user{}
	for i, userName := range userNames {
		users = append(users, &user{
			UserID:   userName,
			Email:    fmt.Sprintf("%s@%s.com", userName, userName),
			Password: userName,
			Mobile:   fmt.Sprintf("08999999%02d", i),
			Database: readDatabaseFromEnv(),
		})
	}
	return users
}

func readDatabaseFromEnv() string {
	db := os.Getenv("CUSTOMER_DATABASE")
	if len(db) == 0 {
		db = "boodabest-login"
	}
	return db
}

func readPAMAuthTokenFromEnv() string {
	token := os.Getenv("PAM_AUTH_TOKEN")
	if len(token) == 0 {
		token = "no-default-token"
	}
	return token
}
