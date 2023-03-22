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
	"encoding/json"
	"errors"
	paho "github.com/eclipse/paho.mqtt.golang"
	"log"
	"strings"
)

func (this *Client) ListenToDeviceCommands(deviceId string, commandHandler DeviceCommandHandler) error {
	if !this.mqtt.IsConnected() {
		log.Println("WARNING: mqtt client not connected")
		return errors.New("mqtt client not connected")
	}
	topic := "command/" + deviceId + "/+"

	handler := func(client paho.Client, message paho.Message) {
		if this.debug {
			log.Println("get command: \n", string(message.Payload()))
		}
		parts := strings.Split(message.Topic(), "/")
		serviceId := parts[len(parts)-1]

		command := Command{}
		err := json.Unmarshal(message.Payload(), &command)
		if err != nil {
			log.Println("ERROR: unable to unmarshal command", err)
			this.SendClientError("unable to unmarshal command: " + err.Error())
			return
		}
		commandHandler(deviceId, serviceId, command)
	}

	token := this.mqtt.Subscribe(topic, 2, handler)
	if token.Wait() && token.Error() != nil {
		log.Println("Error on Subscribe: ", topic, token.Error())
		return token.Error()
	}

	this.registerSubscription(topic, handler)
	return nil
}

func (this *Client) ListenToCommands(commandHandler DeviceCommandHandler) error {
	if !this.mqtt.IsConnected() {
		log.Println("WARNING: mqtt client not connected")
		return errors.New("mqtt client not connected")
	}
	topic := "command/+/+"

	handler := func(client paho.Client, message paho.Message) {
		if this.debug {
			log.Println("get command: \n", string(message.Payload()))
		}
		parts := strings.Split(message.Topic(), "/")
		deviceId := strings.Join(parts[1:len(parts)-1], "/")
		serviceId := parts[len(parts)-1]

		command := Command{}
		err := json.Unmarshal(message.Payload(), &command)
		if err != nil {
			log.Println("ERROR: unable to unmarshal command", err)
			this.SendClientError("unable to unmarshal command: " + err.Error())
			return
		}
		commandHandler(deviceId, serviceId, command)
	}

	token := this.mqtt.Subscribe(topic, 2, handler)
	if token.Wait() && token.Error() != nil {
		log.Println("Error on Subscribe: ", topic, token.Error())
		return token.Error()
	}

	this.registerSubscription(topic, handler)
	return nil
}

func (this *Client) StopListenToDeviceCommands(deviceId string) error {
	if !this.mqtt.IsConnected() {
		log.Println("WARNING: mqtt client not connected")
		return errors.New("mqtt client not connected")
	}
	topic := "command/" + deviceId + "/+"
	token := this.mqtt.Unsubscribe(topic)
	if token.Wait() && token.Error() != nil {
		log.Println("Error on Unsubscribe: ", topic, token.Error())
		return token.Error()
	}
	this.unregisterSubscriptions(topic)
	return nil
}
