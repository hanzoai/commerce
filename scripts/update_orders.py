#!/usr/bin/env python
import requests
import os

from util.export import Export, json
from util import shipwire

if __name__ == '__main__':
    if not os.path.exists('_export/shipwire.json'):
        print 'Fetching latest orders from Shipwire...'
        shipwire.write_cache()

    s_orders = {x['orderNo']: x for x in shipwire.read_cache()}

    for order in s_orders:
        if order['status'] == "cancelled":
            continue

        req = {
            "attempt": 1,
            "body": {
                "status": 200,
                "message": "Successful",
                "resourceLocation": "",
                "resource": order,
            },
            "timestamp": "2017-02-13T13:31:29-08:00",
            "topic": "order.updated",
            "uniqueEventID": "424242424242",
            "webhookSubscriptionID": 42,
        }
        requests.post("https://api.hanz.io/shipwire/stoned", json=req)
