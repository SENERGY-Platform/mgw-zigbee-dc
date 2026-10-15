<a href="https://github.com/SENERGY-Platform/mgw-zigbee-dc/actions/workflows/tests.yml" rel="nofollow">
    <img src="https://github.com/SENERGY-Platform/mgw-zigbee-dc/actions/workflows/tests.yml/badge.svg?branch=main" alt="Tests" />
</a>

TODO

## Auth
The 'auth_endpoint' config field can/should be left empty if this component is used in a MGW with API-Proxy

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
### Device Attributes

Set on every device this connector reports to the device-management-service, but only if the matching config field or environment variable is set (Go duration, e.g. `1h`, `24h`; not `7d`). An invalid duration stops the connector at start.

- last_message_max_age
  - env: `LAST_MESSAGE_MAX_AGE`
  - read by connection-check-v2: a device whose last message is older counts as offline

The device-cloud-connector writes it to the cloud device with its own attribute origin, so the value stored on the device takes precedence over the account's default attributes in the device-repository.
Offline notifications additionally need the device attribute `monitor_connection_state`, which this connector does not set; it can be set per account as a default attribute in the device-repository.
