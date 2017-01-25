#!/usr/bin/env python
import argparse
import csv
import sys
import re
import json
from collections import defaultdict, namedtuple

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
        'Status': str,
        'Total': int,
        'Test': Bool,
        'UserId': str,
    }

if __name__ == '__main__':
    parser = argparse.ArgumentParser()

    parser.add_argument('--orders', action='store', help='orders CSV export from datastore')
    parser.add_argument('--users', action='store', help='users CSV export from datastore')

    args = parser.parse_args()

    # Read orders and organize by user id for easy reference
    orders = Orders(args.orders).read_csv()
    users = Users(args.users).read_csv()

    contacts = set()
    with open("reamaze_export.json") as f:
        for line in f:
            contact = json.loads(line)
            email = contact.get('email', None)
            if email:
                contacts.add(email)

    with open('reamaze-customers.csv', 'w') as f:
        writer = csv.writer(f)
        writer.writerow(['OrderId', 'Email', 'FirstName', 'LastName',
                         'Quantity', 'Batch'])

        for order in orders.values():
            # Skip test orders
            if order.test:
                continue
            if order.total == 50:
                continue

            # Get batch number
            batch = 1
            if order.metadata_ == '{"batch":"2"}':
                batch = 2

            # Only match customers in reamaze database
            user = users[order.user_id]
            if user.email not in contacts:
                continue

            # Get quantity ordered
            quantity = 0
            for item in order.items_:
                quantity  += item['quantity']

            writer.writerow([order.id, user.email, user.first_name, user.last_name,
                             quantity, batch])
