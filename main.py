import asyncio
import json
from datetime import datetime

import obd
import paho.mqtt.client as mqtt
import pint
import websocket
from obd import OBDResponse
from obd.OBDResponse import Status


class ResponseData:
    command: str | None
    value: str | float | int | None
    timestamp = None
    unit: str | None

    def __init__(
        self,
        command: str = None,
        value: str | float | int = None,
        timestamp=None,
        unit: str = None,
    ):
        self.command = command
        self.value = str(value)

        if isinstance(value, pint.Quantity):
            self.unit = str(value.units)
            self.value = str(value.magnitude)
        elif isinstance(value, Status):
            self.value = str(
                [1 if value.MIL else 0, value.DTC_count, value.ignition_type]
            )
        elif isinstance(value, bytearray):
            self.value = value.decode()

        self.timestamp = (
            timestamp
            if type(timestamp) is datetime
            else datetime.fromtimestamp(timestamp)
        )
        self.unit = unit

    def to_json(self):
        return json.dumps(
            {
                "c": self.command,
                "v": self.value,
                "t": self.timestamp.isoformat(),
                "u": self.unit,
            }
        )


class MQTTHandler:
    client: mqtt

    def __init__(self, endpoint: str) -> None:
        def on_connect(client, userdata, flags, reason_code, properties):
            print(f"Connected with result code {reason_code}")
            client.subscribe("$SYS/#")

        # The callback for when a PUBLISH message is received from the server.
        def on_message(client, userdata, msg):
            print(f"{msg.topic} {str(msg.payload)}")

        self.client = mqtt.Client(mqtt.CallbackAPIVersion.VERSION2, "go_mqtt_client")
        self.client.on_connect = on_connect
        self.client.on_message = on_message
        self.client.username_pw_set(username="robbiebyrd", password="ButStayWok3!")
        self.client.tls_set("./emqxsl-ca.crt")
        self.client.connect(endpoint, 8883, 60)

    def connect(self):
        self.client.loop_forever()

    def send_message(self, message: ResponseData):
        self.client.publish("/car1", message.to_json())


class WSHandler:
    client: websocket.WebSocketApp

    def __init__(self, endpoint: str) -> None:
        # websocket.enableTrace(True)
        # self.client = websocket.WebSocketApp(endpoint)
        # rel.dispatch()
        # self.connect()
        return

    def connect(self):
        # self.client.run_forever(dispatcher=rel, reconnect=5)
        return

    def send_message(self, message):
        # self.client.send(message)
        return


def obd_response_to_dict(response: OBDResponse):
    if response.is_null():
        return None

    return ResponseData(
        response.command.name,
        response.value,
        response.time,
        str(response.value.unit) if hasattr(response.value, "unit") else None,
    )


class MessageHandler:
    watcher: obd.asynchronous.Async

    def __init__(
        self, port: str, baud_rate: int, ws: WSHandler, mq: MQTTHandler
    ) -> None:
        self.watcher = obd.Async(port, baud_rate)

        self._ws = ws
        self._mqtt = mq

    def submit_data_point(self, d):
        parsed_data_point = obd_response_to_dict(d)
        if parsed_data_point is not None:
            try:
                self._ws.send_message(parsed_data_point)
                self._mqtt.send_message(parsed_data_point)
            except Exception as e:
                print(e)


handler = MessageHandler(
    "/dev/rfcomm0",
    38400,
    WSHandler("ws://localhost:8001/"),
    MQTTHandler("x6d01861.ala.us-east-1.emqxsl.com"),
)

for cmd in handler.watcher.supported_commands:
    print(cmd.name, cmd.desc)
    handler.watcher.watch(obd.commands[cmd.name], callback=handler.submit_data_point)

handler.watcher.start()
loop = asyncio.new_event_loop()
loop.run_forever()
