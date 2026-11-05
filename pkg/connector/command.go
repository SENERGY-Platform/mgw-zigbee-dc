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

package connector

import (
	"context"
	"sync"

	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/mgw"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
)

func (this *Connector) startCommandHandling(ctx context.Context, wg *sync.WaitGroup) error {
	wg.Add(1)
	go func() {
		defer wg.Done()
		this.config.GetLogger().Info("start command handling")
		for {
			select {
			case <-ctx.Done():
				this.config.GetLogger().Info("stop command handling")
				return
			case command := <-this.commandbuffer:
				this.handleCommand(command)
			}
		}
	}()
	return nil
}

func (this *Connector) handleCommand(command CommandDesc) {
	ieee, ok := this.getDeviceIeeeAddress(command.DeviceId)
	if !ok {
		return
	}
	switch command.ServiceId {
	case "set":
		this.handleSet(ieee, command)
	case "get":
		msg, err := this.zigbee.GetByIeee(ieee)
		if err != nil {
			this.mgw.SendCommandError(command.Command.CommandId, "unable to get zigbee-value: "+err.Error())
			return
		}
		err = this.mgw.Respond(command.DeviceId, command.ServiceId, mgw.Command{
			CommandId: command.Command.CommandId,
			Data:      string(msg),
		})
		if err != nil {
			this.config.GetLogger().Error("unable to send get result", "error", err)
			this.mgw.SendCommandError(command.Command.CommandId, "unable to send get result: "+err.Error())
		}
	default:
		this.mgw.SendCommandError(command.Command.CommandId, "use of unknown service: "+command.ServiceId)
	}
}

// handleSet publishes the set command. If the acknowledgement is enabled and the payload has comparable keys,
// the response is sent by the ack registry as soon as zigbee2mqtt publishes a matching state;
// otherwise it is sent immediately and empty.
func (this *Connector) handleSet(ieee string, command CommandDesc) {
	var cancelAck func() bool
	if this.acks != nil {
		if expected, ok := ackExpectation(command.Command.Data, this.getExposes(ieee)); ok {
			// registered before publishing, so that a fast state can not be missed
			cancelAck = this.acks.Register(ieee, command.DeviceId, command.ServiceId, command.Command.CommandId, expected)
		}
	}
	err := this.zigbee.SetByIeee(ieee, []byte(command.Command.Data))
	if err != nil {
		// a state that arrived meanwhile has already answered the command; no second answer then
		if cancelAck != nil && !cancelAck() {
			this.config.GetLogger().Error("unable to send command to zigbee-mqtt, command already answered", "error", err)
			return
		}
		this.mgw.SendCommandError(command.Command.CommandId, "unable to send command to zigbee-mqtt: "+err.Error())
		return
	}
	if cancelAck != nil {
		return
	}
	err = this.mgw.Respond(command.DeviceId, command.ServiceId, mgw.Command{
		CommandId: command.Command.CommandId,
		Data:      "",
	})
	if err != nil {
		this.config.GetLogger().Error("unable to send empty response", "error", err)
		this.mgw.SendCommandError(command.Command.CommandId, "unable to send empty response: "+err.Error())
	}
}

// getExposes returns the exposes of the device definition; nil if the device or its definition is not available
func (this *Connector) getExposes(ieee string) []model.ZigbeeDeviceInfoFeature {
	devices, err := this.zigbee.GetDeviceList(nil)
	if err != nil {
		return nil
	}
	for _, device := range devices {
		if device.IeeeAddress == ieee {
			return device.Definition.Exposes
		}
	}
	return nil
}
