#!/usr/bin/env python
import argparse
import csv
import sys
import re
from collections import defaultdict, namedtuple

csv.field_size_limit(sys.maxsize)


def snake(name):
    s1 = re.sub('(.)([A-Z][a-z]+)', r'\1_\2', name)
    return re.sub('([a-z0-9])([A-Z])', r'\1_\2', s1).lower()


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
            if typ != str:
                attrs.append(typ(prop))
            else:
                attrs.append(prop)

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
        'UserId': str,
        'PaymentStatus': str,
        'Test': Bool,
        'Total': int,
    }

    def ignore(self, order):
        if order.total == 50:
            return True
        if order.test:
            return True
        if order.payment_status != 'paid':
            return True

class Referrals(Export):
    name = 'Referral'
    fields = {
        'ReferrerUserId': str,
        'OrderId': str,
    }


if __name__ == '__main__':
    parser = argparse.ArgumentParser()

    parser.add_argument('--orders', action='store', help='orders CSV export from datastore')
    parser.add_argument('--referrals', action='store', help='referrals CSV export from datastore')
    parser.add_argument('--users', action='store', help='users CSV export from datastore')

    args = parser.parse_args()

    # Read orders and organize by user id for easy reference
    orders = Orders(args.orders).read_csv()
    orders_by_userid = defaultdict(list)
    for order in orders.values():
        orders_by_userid[order.user_id].append(order)

    # Read users
    users = Users(args.users).read_csv()

    # Read referrals
    referrals = Referrals(args.referrals).read_csv()

    # Find all users that have referred at least 5 orders that also purchased
    # earphones
    referrers = defaultdict(list)
    for referral in referrals.values():
        if referral.referrer_user_id in orders_by_userid and referral.order_id in orders:
            referrers[referral.referrer_user_id].append(referral)

    with open('referrers.csv','w') as f:
        writer = csv.writer(f)
        writer.writerow(['UserId', 'FullName', 'Email', 'Referrals'])
        for referrer, referrals in referrers.items():
            if len(referrals) > 4:
                writer.writerow([referrer, users[referrer].first_name + ' ' +
                                 users[referrer].last_name, users[referrer].email,
                                 len(referrals)])
