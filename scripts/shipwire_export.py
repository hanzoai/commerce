#!/usr/bin/env python
from shipwire import *
import json

def export_orders():
    s = Shipwire(username='dev@hanzo.ai',
                 password='',
                 host='api.shipwire.com')

    r = s.orders.list()

    with open('shipwire.json', 'w') as f:
        for order in r.all_serial():
            f.write(json.dumps(order['resource']) + '\n')

if __name__ == '__main__':
    export_orders()
