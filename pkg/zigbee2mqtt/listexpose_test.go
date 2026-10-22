/*
 * Copyright (c) 2026 InfAI (CC SES)
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
	"log/slog"
	"reflect"
	"testing"

	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/tests/resources"
)

func TestParseDeviceInfoMessageWithListExpose(t *testing.T) {
	devices, err := parseDeviceInfoMessage(resources.DeviceInfoListExposeJson, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 3 {
		t.Fatalf("expected 3 devices, got %v", len(devices))
	}
	if devices[2].Definition.Model != "BTH-RM230Z" {
		t.Fatalf("unexpected model %v", devices[2].Definition.Model)
	}

	msg := jsonNormalize(GetZigbeeMessageStruct(devices[2], ZigbeeSetAccess))
	expected := jsonNormalize(map[string]interface{}{
		"local_temperature_calibration": 5,
		"occupied_heating_setpoint":     30,
		"system_mode":                   "off",
		"weekly_schedule": map[string]interface{}{
			"dayofweek": []interface{}{
				map[string]interface{}{"day": "monday"},
			},
			"transitions": []interface{}{
				map[string]interface{}{
					"transition_time": map[string]interface{}{"hour": 42, "minute": 42},
					"heat_setpoint":   42,
				},
			},
		},
	})
	if !reflect.DeepEqual(expected, msg) {
		t.Errorf("\n%#v\n%#v\n", expected, msg)
	}
}

func TestGetZigbeeMessageStructOfNonCompositeList(t *testing.T) {
	features := []model.ZigbeeDeviceInfoFeature{{
		Type:     "list",
		Property: "values",
		Access:   2,
		ItemType: &model.ZigbeeDeviceInfoFeature{Type: "numeric", Name: "value", Access: 2},
	}}
	msg := jsonNormalize(GetZigbeeMessageStructOfFeatureList(features, ZigbeeSetAccess))
	expected := jsonNormalize(map[string]interface{}{"values": []interface{}{42}})
	if !reflect.DeepEqual(expected, msg) {
		t.Errorf("\n%#v\n%#v\n", expected, msg)
	}
}
