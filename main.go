package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo"
	"github.com/parnurzeal/gorequest"
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
	ConsentIDs string `json:"consent_ids"` // comma separated valud of consent allow ids
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
}

func initVariables() {
	e = echo.New()
	users = readUsersFromEnv()
	req = gorequest.New()
}

func registerServices() error {
	e.POST("/login", handleLogin)
	e.POST("/register", handleRegister)
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

	_, body, err := postRegisterEventToPAMTracker(endpoint, authToken, database, hashedCustID, registerBody.ConsentIDs, user.Email, registerBody.Mobile)
	logMessage(body)

	res := map[string]string{}
	responseSuccess(ctx, res)

	return nil
}

func hashCustomerID(customerID string) string {
	return customerID
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

}

func responseSuccess(ctx echo.Context, obj interface{}) {

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
		"event": "register_success",
		"form_fields": map[string]string{
			"_database":   database,
			"_consent_id": consentAllowID,
			"customer":    customerID,
			"email":       email,
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
	req.Header.Add("Authentication", authToken)
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
	return []*user{
		{
			UserID:   "a",
			Email:    "a@a.com",
			Password: "a",
			Mobile:   "0899999991",
			Database: readDatabaseFromEnv(),
		},
		{
			UserID:   "b",
			Email:    "b@b.com",
			Password: "b",
			Mobile:   "0899999992",
			Database: readDatabaseFromEnv(),
		},
		{
			UserID:   "c",
			Email:    "c@c.com",
			Password: "c",
			Mobile:   "0899999993",
			Database: readDatabaseFromEnv(),
		},
	}
}

func readDatabaseFromEnv() string {
	db := os.Getenv("database")
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
