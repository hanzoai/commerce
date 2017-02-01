#!/usr/bin/env python
import csv
import sys
import re
import json
from collections import defaultdict, namedtuple
from shipstation.api import *

csv.field_size_limit(sys.maxsize)

def snake(name):
    s1 = re.sub('(.)([A-Z][a-z]+)', r'\1_\2', name)
    s2 = re.sub('([a-z0-9])([A-Z])', r'\1_\2', s1).lower()
    s3 = s2.replace('.', '')
    return s3


class RowParser(object):
    def __init__(self, Tuple, header, fields):
        self.Tuple = Tuple
        self.header      = header
        self.fields      = fields

    def get_column(self, row, field):
        idx = self.header[field]
        return row[idx]

    def parse(self, row):
        attrs = [self.get_column(row, 'Id_')]

        for field, typ in self.fields.items():
            prop = self.get_column(row, field)
            if typ == str:
                attrs.append(prop)
            elif typ == json:
                attrs.append(json.loads(prop))
            else:
                attrs.append(typ(prop))

        return self.Tuple(*attrs)


def Bool(b):
    if b == 'True':
        return True
    elif b == 'False':
        return False
    else:
        raise Exception('Invalid boolean: {0}'.format(b))


class Export(object):
    name   = 'Export'
    fields = {}

    def __init__(self, filename):
        self.filename = filename
        self.header   = self.parse_header()
        self.Tuple    = namedtuple(self.name, ['id'] + [snake(f) for f in
                                                           self.fields])

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
    name   = 'User'
    fields = {
        'FirstName': str,
        'LastName': str,
        'Email': str,
    }


class Orders(Export):
    name   = 'Order'
    fields = {
        'Items_': json,
        'Metadata_': str,
        'Paid': int,
        'ShippingAddress.Country': lambda x: x.lower(),
        'ShippingAddress.City': lambda x: x.lower(),
        'Status': str,
        'Total': int,
        'Test': Bool,
        'UserId': str,
    }

class HiPri(Export):
    name   = 'HiPri'
    fields = {
        'Number': str,
        'Email': str,
    }

def get_shipstation_orders():
    API_KEY    = '09b9c22e499748f0a0d32a56ba09a1ba'
    API_SECRET = '7129b81e1f3f49dfa2aee85e20623cbf'

    s = ShipStation(key=API_KEY, secret=API_SECRET)
    s.debug = True

    ss_orders = {}
    page = 1
    res = s.list_orders(pageSize=500)

    # Store indexed by orderNumber
    for order in res['orders']:
        ss_orders[order["orderKey"]] = order

    # Fetch all pages
    while page <= res['pages']:
        page += 1
        res = s.list_orders(page=page, pageSize=500)
        for order in res['orders']:
            ss_orders[order["orderKey"]] = order

    return ss_orders

if __name__ == '__main__':
    # Read orders and organize by user id for easy reference
    orders = Orders('_export/stoned-order-crowdstart-us-2017-02-01.csv').read_csv()
    hipri  = HiPri('hipri-customers.csv').read_csv()
    ss_orders = get_shipstation_orders()

    total = 0
    for id, order in orders.items():
        if order.status != 'open':
            continue

        if id not in hipri:
            continue

        ss_order = ss_orders[id]
        city = ss_order['shipTo']['city'].lower()
        city2 = order.shipping_address_city.lower()
        if city != city2:
            print ss_order[id]['id']
    print total
