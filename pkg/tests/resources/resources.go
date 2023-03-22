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

package resources

import (
	_ "embed"
	"encoding/json"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"log"
	"runtime/debug"
)

//go:embed device_info_example.json
var DeviceInfoExampleJson []byte

var DeviceInfoExample []model.ZigbeeDeviceInfo

func init() {
	err := json.Unmarshal(DeviceInfoExampleJson, &DeviceInfoExample)
	if err != nil {
		log.Println("ERROR:", err)
		debug.PrintStack()
	}
}
