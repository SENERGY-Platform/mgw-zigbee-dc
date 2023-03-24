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

package mocks

import (
	"context"
	"encoding/json"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	paho "github.com/eclipse/paho.mqtt.golang"
	"log"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

type ZigbeeMock struct {
	config configuration.Config
	mqtt   paho.Client
	values map[string]map[string]interface{}
	mux    sync.Mutex
}

func NewZigbeeMock(ctx context.Context, wg *sync.WaitGroup, config configuration.Config) (*ZigbeeMock, error) {
	client := &ZigbeeMock{
		config: config,
		values: map[string]map[string]interface{}{},
	}
	options := paho.NewClientOptions().
		SetPassword(client.config.ZigbeeMqttPw).
		SetUsername(client.config.ZigbeeMqttUser).
		SetAutoReconnect(true).
		SetCleanSession(true).
		SetClientID(client.config.ZigbeeMqttClientId + "_mock_server").
		AddBroker(client.config.ZigbeeMqttBroker).
		SetWriteTimeout(10 * time.Second).
		SetOrderMatters(false).
		SetConnectionLostHandler(func(_ paho.Client, err error) {
			log.Println("connection to zigbee2mqtt broker lost")
		}).
		SetOnConnectHandler(func(_ paho.Client) {
			log.Println("connected to zigbee2mqtt broker")
			err := client.startListener()
			if err != nil {
				log.Fatal("FATAL: ", err)
			}
		})

	client.mqtt = paho.NewClient(options)
	if token := client.mqtt.Connect(); token.Wait() && token.Error() != nil {
		log.Println("Error on MqttStart.Connect(): ", token.Error())
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

func (this *ZigbeeMock) startListener() error {
	token := this.mqtt.Subscribe(this.config.ZigbeeMqttTopicPrefix+"+/set", 2, func(client paho.Client, message paho.Message) {
		go func() {
			device := strings.TrimPrefix(strings.TrimSuffix(message.Topic(), "/set"), this.config.ZigbeeMqttTopicPrefix)
			this.setValue(device, message.Payload())
			event := this.getValue(device)
			t := this.mqtt.Publish(this.config.ZigbeeMqttTopicPrefix+device, 2, false, event)
			t.Wait()
			err := t.Error()
			if err != nil {
				log.Println("ERROR:", err)
				debug.PrintStack()
			}
		}()
	})
	token.Wait()
	err := token.Error()
	if err != nil {
		log.Println("ERROR:", err)
		debug.PrintStack()
	}

	token = this.mqtt.Subscribe(this.config.ZigbeeMqttTopicPrefix+"+/get", 2, func(client paho.Client, message paho.Message) {
		go func() {
			device := strings.TrimPrefix(strings.TrimSuffix(message.Topic(), "/get"), this.config.ZigbeeMqttTopicPrefix)
			event := this.getValue(device)
			t := this.mqtt.Publish(this.config.ZigbeeMqttTopicPrefix+device, 2, false, event)
			t.Wait()
			err := t.Error()
			if err != nil {
				log.Println("ERROR:", err)
				debug.PrintStack()
			}
		}()
	})
	token.Wait()
	err = token.Error()
	if err != nil {
		log.Println("ERROR:", err)
		debug.PrintStack()
	}
	return nil
}

func (this *ZigbeeMock) setValue(device string, payload []byte) {
	temp := map[string]interface{}{}
	err := json.Unmarshal(payload, &temp)
	if err != nil {
		log.Println("ERROR:", err)
		debug.PrintStack()
		return
	}
	this.setValueObj(device, temp)
}

func (this *ZigbeeMock) setValueObj(device string, obj map[string]interface{}) {
	this.mux.Lock()
	defer this.mux.Unlock()
	current, ok := this.values[device]
	if !ok {
		current = map[string]interface{}{}
	}
	for k, v := range obj {
		current[k] = v
	}
	this.values[device] = current
}

func (this *ZigbeeMock) getValue(device string) []byte {
	current, ok := this.values[device]
	if !ok {
		current = map[string]interface{}{}
	}
	result, err := json.Marshal(current)
	if err != nil {
		log.Println("ERROR:", err)
		debug.PrintStack()
	}
	return result
}

func (this *ZigbeeMock) TriggerEvent(device string, value map[string]interface{}) {
	this.setValueObj(device, value)
	t := this.mqtt.Publish(this.config.ZigbeeMqttTopicPrefix+device, 2, false, this.getValue(device))
	t.Wait()
	err := t.Error()
	if err != nil {
		log.Println("ERROR:", err)
		debug.PrintStack()
	}
}

func (this *ZigbeeMock) SendDeviceInfos(infos []model.ZigbeeDeviceInfo) {
	temp, err := json.Marshal(infos)
	if err != nil {
		log.Println("ERROR:", err)
		debug.PrintStack()
		return
	}
	t := this.mqtt.Publish(this.config.ZigbeeMqttTopicPrefix+"bridge/devices", 2, true, temp)
	t.Wait()
	err = t.Error()
	if err != nil {
		log.Println("ERROR:", err)
		debug.PrintStack()
	}
}
