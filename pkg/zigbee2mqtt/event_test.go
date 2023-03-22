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
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/tests/resources"
	"reflect"
	"testing"
)

func TestGetZigbeeGetMessage(t *testing.T) {
	msg := getZigbeeGetMessage(resources.DeviceInfoExample[1])
	expected := map[string]interface{}{
		"brightness":         nil,
		"color":              nil,
		"color_temp":         nil,
		"color_temp_startup": nil,
		"power_on_behavior":  nil,
		"state":              nil,
	}

	if !reflect.DeepEqual(expected, msg) {
		t.Errorf("\n%#v\n%#v\n", expected, msg)
	}
}

func Test_checkZigbeeAccess(t *testing.T) {
	type args struct {
		access   int
		checkFor ZigbeeAccess
	}
	tests := []struct {
		args args
		want bool
	}{
		{
			args: args{access: 1, checkFor: ZigbeeGetAccess},
			want: false,
		},
		{
			args: args{access: 1, checkFor: ZigbeeSetAccess},
			want: false,
		},
		{
			args: args{access: 1, checkFor: ZigbeePublishAccess},
			want: true,
		},
		{
			args: args{access: 7, checkFor: ZigbeeGetAccess},
			want: true,
		},
		{
			args: args{access: 7, checkFor: ZigbeeSetAccess},
			want: true,
		},
		{
			args: args{access: 7, checkFor: ZigbeePublishAccess},
			want: true,
		},
		{
			args: args{access: 5, checkFor: ZigbeeGetAccess},
			want: true,
		},
		{
			args: args{access: 5, checkFor: ZigbeeSetAccess},
			want: false,
		},
		{
			args: args{access: 5, checkFor: ZigbeePublishAccess},
			want: true,
		},
	}
	for _, tt := range tests {
		if got := checkZigbeeAccess(tt.args.access, tt.args.checkFor); got != tt.want {
			t.Errorf("checkZigbeeAccess() = %v, want %v", got, tt.want)
		}
	}
}
