package nats_client

import (
	"os"
	"strings"
	"time"

	"github.com/cloudfoundry/yagnats"
	"github.com/pivotal-golang/lager"
)

type Runner struct {
	client    yagnats.NATSClient
	addresses string
	username  string
	password  string
	logger    lager.Logger
}

func New(addresses, username, password string, logger lager.Logger) Runner {
	return Runner{
		client:    yagnats.NewClient(),
		addresses: addresses,
		username:  username,
		password:  password,
		logger:    logger.Session("nats-runnner"),
	}
}

func (c Runner) Client() yagnats.NATSClient {
	return c.client
}

func (c Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	natsMembers := []yagnats.ConnectionProvider{}

	for _, addr := range strings.Split(c.addresses, ",") {
		natsMembers = append(
			natsMembers,
			&yagnats.ConnectionInfo{
				Addr:     addr,
				Username: c.username,
				Password: c.password,
			},
		)
	}

	config := &yagnats.ConnectionCluster{
		Members: natsMembers,
	}

	err := c.client.Connect(config)
	for err != nil {
		c.logger.Error("connecting-to-nats-failed", err)
		select {
		case <-signals:
			return nil
		case <-time.After(time.Second):
			err = c.client.Connect(config)
		}
	}

	c.logger.Info("connecting-to-nats-succeeeded")
	close(ready)

	<-signals
	return nil
}
