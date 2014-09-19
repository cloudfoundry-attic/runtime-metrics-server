package nats_client

import (
	"net/url"
	"strings"

	"github.com/cloudfoundry/yagnats"
)

func New(addresses, username, password string) yagnats.ApceraWrapperNATSClient {
	natsMembers := []string{}
	for _, addr := range strings.Split(addresses, ",") {
		uri := url.URL{
			Scheme: "nats",
			User:   url.UserPassword(username, password),
			Host:   addr,
		}
		natsMembers = append(natsMembers, uri.String())
	}

	return yagnats.NewApceraClientWrapper(natsMembers)
}
