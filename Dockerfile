FROM 3dsinteractive/golang:1.14-alpine3.9

WORKDIR /go/src/bitbucket.org/3dsinteractive/pam-demo-server
ADD . /go/src/bitbucket.org/3dsinteractive/pam-demo-server
RUN go build -mod vendor -i -tags "musl" main.go

# ================
FROM 3dsinteractive/alpine:3.9

COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=0 /go/src/bitbucket.org/3dsinteractive/pam-demo-server/main /main

ADD ./entrypoint.sh /entrypoint.sh

RUN adduser -u 1001 -D -s /bin/sh -G ping 1001
RUN chown 1001:1001 /entrypoint.sh
RUN chown 1001:1001 /main

RUN chmod +x /entrypoint.sh
RUN chmod +x /main

USER 1001

EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
