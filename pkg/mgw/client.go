/*
 * Copyright (c) 2023 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mgw

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
	paho "github.com/eclipse/paho.mqtt.golang"
)

const DeviceManagerTopic = "device-manager/device"

type Client struct {
	mqtt                         paho.Client
	debug                        bool
	connectorId                  string
	subscriptions                map[string]paho.MessageHandler
	subscriptionsMux             sync.Mutex
	deviceManagerRefreshNotifier func()
}

func New(ctx context.Context, wg *sync.WaitGroup, config configuration.Config, refreshNotifier func()) (*Client, error) {
	client := &Client{
		connectorId:                  config.ConnectorId,
		debug:                        config.Debug,
		deviceManagerRefreshNotifier: refreshNotifier,
		subscriptions:                map[string]paho.MessageHandler{},
	}
	lwt := "device-manager/device/" + config.ConnectorId + "/lw"
	options := paho.NewClientOptions().
		SetPassword(config.MgwMqttPw).
		SetUsername(config.MgwMqttUser).
		SetAutoReconnect(true).
		SetCleanSession(true).
		SetClientID(config.MgwMqttClientId).
		AddBroker(config.MgwMqttBroker).
		SetWriteTimeout(10*time.Second).
		SetOrderMatters(false).
		SetResumeSubs(true).
		SetConnectionLostHandler(func(_ paho.Client, err error) {
			config.GetLogger().Error("connection to mgw broker lost", "error", err)
		}).
		SetOnConnectHandler(func(_ paho.Client) {
			config.GetLogger().Info("connected to mgw broker")
			err := client.initSubscriptions()
			if err != nil {
				config.GetLogger().Error("fatal: unable to init subscriptions", "error", err)
				log.Fatal("FATAL: ", err)
			}
			if client.deviceManagerRefreshNotifier != nil {
				client.deviceManagerRefreshNotifier()
			}
		}).SetWill(lwt, "offline", 2, false)

	client.mqtt = paho.NewClient(options)
	if token := client.mqtt.Connect(); token.Wait() && token.Error() != nil {
		config.GetLogger().Error("unable to connect to mgw broker", "error", token.Error())
		return nil, token.Error()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		client.mqtt.Disconnect(200)
	}()

	return client, nil
}

func (this *Client) NotifyDeviceManagerRefresh(f func()) {
	this.deviceManagerRefreshNotifier = f
}
