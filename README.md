<a href="https://github.com/SENERGY-Platform/mgw-zigbee-dc/actions/workflows/tests.yml" rel="nofollow">
    <img src="https://github.com/SENERGY-Platform/mgw-zigbee-dc/actions/workflows/tests.yml/badge.svg?branch=main" alt="Tests" />
</a>

TODO


### Services

#### Set

- local-service-id: "set"
- inputs: json object containing all values that are to be changed
- no outputs

#### Get

- local-service-id: "get"
- no inputs
- outputs: json object with all known device values
  - values/fields may be missing if device has never sent them, and they are not requestable (ref https://www.zigbee2mqtt.io/guide/usage/exposes.html#access)

### Device-Type Attributes

- senergy/zigbee-dc
  - existence indicates that the device-type may be used by this device-connector 
- senergy/zigbee-vendor
  - used to match device-type
  - found in mqtt messages in zigbee2mqtt/bridge/devices 
  - e.g. Philips in $[1].definition.vendor in pkg/tests/resources/device_info_example.json
- senergy/zigbee-model
  - used to match device-type
  - found in mqtt messages in zigbee2mqtt/bridge/devices
  - e.g. 9290012573A in $[1].definition.model in pkg/tests/resources/device_info_example.json