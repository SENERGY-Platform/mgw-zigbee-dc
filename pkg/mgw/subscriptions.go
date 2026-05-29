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
	"errors"
	"log/slog"

	paho "github.com/eclipse/paho.mqtt.golang"
)

func (this *Client) registerSubscription(topic string, handler paho.MessageHandler) {
	this.subscriptionsMux.Lock()
	defer this.subscriptionsMux.Unlock()
	this.subscriptions[topic] = handler
}

func (this *Client) unregisterSubscriptions(topic string) {
	this.subscriptionsMux.Lock()
	defer this.subscriptionsMux.Unlock()
	delete(this.subscriptions, topic)
}

func (this *Client) getSubscriptions() (result []Subscription) {
	this.subscriptionsMux.Lock()
	defer this.subscriptionsMux.Unlock()
	for topic, handler := range this.subscriptions {
		result = append(result, Subscription{Topic: topic, Handler: handler})
	}
	return
}

func (this *Client) initSubscriptions() (err error) {
	err = this.loadOldSubscriptions()
	if err != nil {
		return err
	}
	err = this.listenToDeviceManagementRefresh()
	return nil
}

func (this *Client) loadOldSubscriptions() error {
	if !this.mqtt.IsConnected() {
		slog.Warn("mqtt client not connected")
		return errors.New("mqtt client not connected")
	}
	subs := this.getSubscriptions()
	for _, sub := range subs {
		slog.Debug("resubscribe", "topic", sub.Topic)
		token := this.mqtt.Subscribe(sub.Topic, 2, sub.Handler)
		if token.Wait() && token.Error() != nil {
			slog.Error("Error on Subscribe", "topic", sub.Topic, "error", token.Error())
			return token.Error()
		}
	}
	return nil
}

func (this *Client) listenToDeviceManagementRefresh() error {
	if !this.mqtt.IsConnected() {
		slog.Warn("mqtt client not connected")
		return errors.New("mqtt client not connected")
	}

	topic := "device-manager/refresh"
	handler := func(paho.Client, paho.Message) {
		slog.Debug("received device-manager refresh message")
		if this.deviceManagerRefreshNotifier != nil {
			slog.Debug("notify device-manager refresh message")
			this.deviceManagerRefreshNotifier()
		}
	}

	token := this.mqtt.Subscribe(topic, 2, handler)
	if token.Wait() && token.Error() != nil {
		slog.Error("Error on Subscribe", "topic", topic, "error", token.Error())
		return token.Error()
	}
	return nil
}
