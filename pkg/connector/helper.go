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
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/zigbee2mqtt"
	"sync"
)

// FMap4 transforms a function f with output type A to a function with output type B
// the number 4 stands for 4 inputs
//
// FMap1(time.ParseDuration, func(d time.Duration) int64 {return d.Milliseconds()})
//
// is equivalent to
//
//	func(s string) (int64, error) {
//	   	temp, err := time.ParseDuration(s)
//			if err != nil {
//				return 0, err
//			}
//			return temp.Milliseconds()
//	}
func FMap4[I1 any, I2 any, I3 any, I4 any, ResultType any, NewResultType any](f func(in1 I1, in2 I2, in3 I3, in4 I4) (ResultType, error), c func(ResultType) NewResultType) func(in1 I1, in2 I2, in3 I3, in4 I4) (NewResultType, error) {
	return func(in1 I1, in2 I2, in3 I3, in4 I4) (result NewResultType, err error) {
		temp, err := f(in1, in2, in3, in4)
		if err != nil {
			return result, err
		}
		result = c(temp)
		return result, err
	}
}

func ZigbeeFactoryCast[ResultType ZigbeeClient](f func(context.Context, *sync.WaitGroup, configuration.Config, zigbee2mqtt.Connector) (ResultType, error)) ZigbeeFactory {
	return FMap4(f, func(resultType ResultType) ZigbeeClient {
		return resultType
	})
}

func MgwFactoryCast[ResultType MgwClient](f func(context.Context, *sync.WaitGroup, configuration.Config, func()) (ResultType, error)) MgwFactory {
	return FMap4(f, func(resultType ResultType) MgwClient {
		return resultType
	})
}
