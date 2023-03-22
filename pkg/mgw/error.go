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

import "log"

func (this *Client) SendClientError(message string) {
	if !this.mqtt.IsConnected() {
		log.Println("WARNING: mqtt client not connected")
		return
	}
	payload := this.connectorId + ": " + message
	topic := "error/client"
	if this.debug {
		log.Println("DEBUG: publish ", topic, payload)
	}
	token := this.mqtt.Publish(topic, 2, false, payload)
	if token.Wait() && token.Error() != nil {
		log.Println("Error on Client.Publish(): ", token.Error())
	}
	return
}

func (this *Client) SendDeviceError(localDeviceId string, message string) {
	if !this.mqtt.IsConnected() {
		log.Println("WARNING: mqtt client not connected")
		return
	}
	payload := this.connectorId + ": " + message
	topic := "error/device/" + localDeviceId
	if this.debug {
		log.Println("DEBUG: publish ", topic, payload)
	}
	token := this.mqtt.Publish(topic, 2, false, payload)
	if token.Wait() && token.Error() != nil {
		log.Println("Error on Client.Publish(): ", token.Error())
	}
	return
}

func (this *Client) SendCommandError(correlationId string, message string) {
	if !this.mqtt.IsConnected() {
		log.Println("WARNING: mqtt client not connected")
		return
	}
	payload := this.connectorId + ": " + message
	topic := "error/command/" + correlationId
	if this.debug {
		log.Println("DEBUG: publish ", topic, payload)
	}
	token := this.mqtt.Publish(topic, 2, false, message)
	if token.Wait() && token.Error() != nil {
		log.Println("Error on Client.Publish(): ", token.Error())
	}
	return
}
