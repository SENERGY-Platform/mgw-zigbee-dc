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
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/mgw"
	"log"
	"sync"
)

func (this *Connector) startCommandHandling(ctx context.Context, wg *sync.WaitGroup) error {
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("start command handling")
		for {
			select {
			case <-ctx.Done():
				log.Println("stop command handling")
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
		err := this.zigbee.SetByIeee(ieee, []byte(command.Command.Data))
		if err != nil {
			this.mgw.SendCommandError(command.Command.CommandId, "unable to send command to zigbee-mqtt: "+err.Error())
			return
		}
		err = this.mgw.Respond(command.DeviceId, command.ServiceId, mgw.Command{
			CommandId: command.Command.CommandId,
			Data:      "",
		})
		if err != nil {
			log.Println("ERROR: unable to send empty response", err)
			this.mgw.SendCommandError(command.Command.CommandId, "unable to send empty response: "+err.Error())
		}
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
			log.Println("ERROR: unable to send get result", err)
			this.mgw.SendCommandError(command.Command.CommandId, "unable to send get result: "+err.Error())
		}
	default:
		this.mgw.SendCommandError(command.Command.CommandId, "use of unknown service: "+command.ServiceId)
	}
}
