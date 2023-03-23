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
	"fmt"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/tests/resources"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/zigbee2mqtt"
	"strings"
)

func ExampleGetMissingDeviceTypeMessage() {
	fmt.Print(strings.TrimSpace(GetMissingDeviceTypeMessage(resources.DeviceInfoExample[1], func(device model.ZigbeeDeviceInfo) map[string]interface{} {
		return zigbee2mqtt.GetZigbeeMessageStruct(device, zigbee2mqtt.ZigbeeEventAccess)
	}, func(device model.ZigbeeDeviceInfo) map[string]interface{} {
		return zigbee2mqtt.GetZigbeeMessageStruct(device, zigbee2mqtt.ZigbeeSetAccess)
	})))

	//output:
	//missing zigbee device-type, please provide a device-type with:
	//ref: https://www.zigbee2mqtt.io/devices/9290012573A.html
	//attributes:
	//     - senergy/zigbee-dc = true
	//     - senergy/zigbee-vendor = Philips
	//     - senergy/zigbee-model = 9290012573A
	//services:
	//---------------
	//local-id: get
	//protocol: standard-connector
	//example output data:
	//{
	//     "brightness": 254,
	//     "color": {
	//         "hue": 42,
	//         "saturation": 42,
	//         "x": 42,
	//         "y": 42
	//     },
	//     "color_temp": 500,
	//     "color_temp_startup": 500,
	//     "linkquality": 255,
	//     "power_on_behavior": "off",
	//     "state": "ON"
	//}
	//---------------
	//local-id: set
	//protocol: standard-connector
	//example input data:
	//{
	//     "brightness": 254,
	//     "color": {
	//         "hue": 42,
	//         "saturation": 42,
	//         "x": 42,
	//         "y": 42
	//     },
	//     "color_temp": 500,
	//     "color_temp_startup": 500,
	//     "effect": "blink",
	//     "power_on_behavior": "off",
	//     "state": "ON"
	//}
	//---------------

}
