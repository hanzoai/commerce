#!/usr/bin/env python
from collections import namedtuple
from functools import partial
from itertools import ifilter, imap
from pprint import pprint
from shipwire import *

import argparse
import csv
import json
import os

if False:

    r = s.orders.list()

    with open('shipwire.json', 'w') as f:
        for order in r.all_serial():
            f.write(json.dumps(order['resource']) + '\n')

class RowParser(object):
    def __init__(self, Tuple, header, fields):
        self.Tuple = Tuple
        self.header = header
        self.fields = fields

    def get_column(self, row, field):
        idx = self.header[field]
        return row[idx]

    def parse(self, row):
        attrs = [self.get_column(row, 'Id_')]

        for field, typ in self.fields.items():
            prop = self.get_column(row, field)
            if typ != str:
                attrs.append(typ(prop))
            else:
                attrs.append(prop)

        return self.Tuple(*attrs)


class Export(object):
    name   = 'Export'
    fields = {}

    def __init__(self, filename):
        self.filename = filename
        self.header   = self.parse_header()
        self.Tuple    = namedtuple(self.name, ['id'] + self.fields.keys())

    def parse_header(self):
        with open(self.filename) as f:
            first_row = next(f).split(',')
            return dict((k.strip(),i) for i,k in enumerate(first_row))

    def get_parser(self):
        return RowParser(self.Tuple, self.header, self.fields)

    def ignore(self, obj):
        return False

    def read_csv(self):
        entities = {}

        with open(self.filename) as f:
            next(f)  # Skip header
            parser = self.get_parser()
            for _, row in enumerate(csv.reader(f)):
                obj = parser.parse(row)
                if not self.ignore(obj):
                    entities[obj.id] = obj

        return entities

class Users(Export):
    name = 'User'
    fields = { }

class Orders(Export):
    name = 'Order'
    fields = { 'Status' : str, 'PaymentStatus' : str }

def submit_test_order(sw, order):
    v = {
        'orderNo' : 'filter-shipwire-test',
        'shipTo' : {
            'name' : 'Imran Hameed',
            'address1' : '33 N Almadden Blvd Unit 1601',
            'city' : 'San Jose',
            'state' : 'CA',
            'country' : 'USA',
        },
        'items' : [
            {
                'sku' : 'TestSku1',
                'quantity' : 3,
            }
        ],
    }
    json_v = json.dumps(v)
    pprint(json_v)

    if True:
        resp = sw.order.create(json=v)
        print '######### BEGIN shipwire response ='
        print '######### resp.status'
        pprint(resp.status)
        print '######### resp.message'
        pprint(resp.message)
        print '######### resp.json'
        pprint(resp.json)
        print '######### resp.location'
        pprint(resp.location)
        print '######### resp.warnings'
        pprint(resp.warnings)
        print '######### resp.errors'
        pprint(resp.errors)
        print '######### END shipwire response ='

def shipwire_login():
    return Shipwire(username='dev@hanzo.ai', password='', host='api.shipwire.com')

CliArgs = namedtuple('CliArgs', ['orders', 'users', 'shipwire_orders'])

def parse_cli():
    parser = argparse.ArgumentParser()
    parser.add_argument('--orders', type=str, action='store', nargs=1, required=True, help='path to the crowdstart order csv')
    parser.add_argument('--users', type=str, action='store', nargs=1, required=True, help='path to the crowdstart users csv')
    parser.add_argument('--shipwire-orders', type=str, action='store', nargs=1, required=True, help='path to the shipwire orders json, as produced by scripts/list_shipwire.py')
    args = parser.parse_args()
    return CliArgs(orders=args.orders[0], users=args.users[0], shipwire_orders=args.shipwire_orders[0])

def main():
    args = parse_cli()
    orders = Orders(args.orders).read_csv().values()
    users = Users(args.users).read_csv().values()

    def open_order(order): return order.Status.lower() == 'open' and order.PaymentStatus.lower() == 'paid'
    def canceled_order(order): return order.Status.lower() == 'cancelled' and order.PaymentStatus.lower() == 'refunded'

    open_orders = list(ifilter(open_order, orders))
    canceled_orders = list(ifilter(canceled_order, orders))
    invalid_orders = list(ifilter(lambda order: not open_order(order) and not canceled_order(order), orders))
    print 'open orders: %r; canceled orders: %r, invalid orders: %r, total orders = %r' % (len(open_orders), len(canceled_orders), len(invalid_orders), len(orders))

    sw = shipwire_login()
    submit_test_order(sw, None)

if __name__ == '__main__':
    main()
