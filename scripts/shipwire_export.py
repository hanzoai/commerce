#!/usr/bin/env python
from shipwire import *
import json


def read_cached():
    with open('shipwire.json') as f:
        for line in f:
            yield json.loads(line)


def write_cached():
    s = Shipwire(username='dev@hanzo.ai',
                 password='',
                 host='api.shipwire.com')

    r = s.orders.list()

    with open('shipwire.json', 'w') as f:
        for order in r.all_serial():
            f.write(json.dumps(order['resource']) + '\n')


if __name__ == '__main__':
    write_cached()
