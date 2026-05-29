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

package zigbee2mqtt

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	paho "github.com/eclipse/paho.mqtt.golang"
)

type Client struct {
	mqtt      paho.Client
	debug     bool
	config    configuration.Config
	register  *DeviceRegister
	connector Connector
	valueMux  sync.Mutex
	values    map[string][]byte
}

type Connector interface {
	Event(device model.ZigbeeDeviceInfo, payload []byte)
	DeviceInfoUpdate(devices []model.ZigbeeDeviceInfo)
}

func New(ctx context.Context, wg *sync.WaitGroup, config configuration.Config, connector Connector) (*Client, error) {
	config.GetLogger().Info("start zigbee2mqtt client")
	client := &Client{
		debug:     config.Debug,
		config:    config,
		register:  NewDeviceRegister(),
		values:    map[string][]byte{},
		connector: connector,
	}

	options := paho.NewClientOptions().
		SetPassword(client.config.ZigbeeMqttPw).
		SetUsername(client.config.ZigbeeMqttUser).
		SetAutoReconnect(true).
		SetCleanSession(true).
		SetClientID(client.config.ZigbeeMqttClientId).
		AddBroker(client.config.ZigbeeMqttBroker).
		SetWriteTimeout(10 * time.Second).
		SetOrderMatters(false).
		SetConnectionLostHandler(func(_ paho.Client, err error) {
			config.GetLogger().Error("connection to zigbee2mqtt broker lost", "error", err)
		}).
		SetOnConnectHandler(func(_ paho.Client) {
			config.GetLogger().Info("connected to zigbee2mqtt broker")
			err := client.startListener()
			if err != nil {
				config.GetLogger().Error("fatal: unable to start listener", "error", err)
				log.Fatal("FATAL: ", err)
			}
		})

	client.mqtt = paho.NewClient(options)
	if token := client.mqtt.Connect(); token.Wait() && token.Error() != nil {
		config.GetLogger().Error("unable to connect to zigbee2mqtt broker", "error", token.Error())
		return client, token.Error()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		client.mqtt.Disconnect(200)
	}()

	return client, client.startEventRefreshLoop(ctx)
}

func (this *Client) startEventRefreshLoop(ctx context.Context) (err error) {
	if this.config.EventRefreshInterval != "" && this.config.EventRefreshInterval != "-" {
		dur, err := time.ParseDuration(this.config.EventRefreshInterval)
		if err != nil {
			this.config.GetLogger().Error("unable to parse EventRefreshInterval", "error", err)
			return err
		}
		ticker := time.NewTicker(dur)
		go func() {
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					err := this.RefreshEventValues()
					if err != nil {
						this.config.GetLogger().Error("unable to refresh event values", "error", err)
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	return nil
}

func (this *Client) startListener() error {
	err := this.startDeviceInfoListener()
	if err != nil {
		return err
	}
	err = this.startEventListener()
	if err != nil {
		return err
	}
	return nil
}
