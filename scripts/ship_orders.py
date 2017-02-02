#!/usr/bin/env python
from export import Export, json, latest_csv, to_csv
from datetime import datetime
from shipwire import *
from shipwire_export import read_cached, write_cached


class User(Export):
    fields = {
        'Id_': str,
        'Email': str,
        'FirstName': str,
        'LastName': str,
    }


class Order(Export):
    def __init__(self, filename, users):
        super(Order, self).__init__(filename)
        self.users = users

    fields = {
        # Fields in CSV
        'Id_': str,
        'CreatedAt': lambda x: datetime.strptime(x, '%Y-%m-%dT%H:%M:%S'),
        'Items_': json,
        'Metadata_': json,
        'Status': lambda x: x.lower(),
        'PaymentStatus': lambda x: x.lower(),
        'Test': bool,
        'UserId': str,
        'Number': str,
        'Total': int,
        'ShippingAddress.Name': str,
        'ShippingAddress.Country': lambda x: x.upper(),
        'ShippingAddress.State': str,
        'ShippingAddress.City': str,
        'ShippingAddress.PostalCode': str,
        'ShippingAddress.Line1': str,
        'ShippingAddress.Line2': str,

        # Not in CSV, populated from user
        'Email': None,
        'FirstName': None,
        'LastName': None,
        'Batch': None,
    }

    def ignore(self, order):
        return order.test or order.total == 50

    def hydrate(self, order):
        def determine_batch(order):
            batch = order.metadata_['batch']
            if batch == '2':
                return 2
            elif batch == 'f2k':
                return 'f2k'
            else:
                return 1

        # Process batch metadata
        order.batch = determine_batch(order)

        # Hydrate order with user data
        user = self.users[order.user_id]
        order.email      = user.email
        order.first_name = user.first_name
        order.last_name  = user.last_name

        return order


def get_orders():
    # Read out processed orders from shipwire local copy
    ss_orders = set(x['orderNo'] for x in read_cached() if x['status'] != 'cancelled')

    def open(order):
        return order.status == 'open' and order.payment_status == 'paid'

    def cancelled(order):
        return order.status == 'cancelled' or order.payment_status == 'refunded'

    def locked(order):
        return order.status == 'locked'

    def disputed(order):
        return order.payment_status == 'disputed'

    def invalid(order):
        return not open(order) and not cancelled(order) and not disputed(order)

    def domestic(order):
        return order.shipping_address_country == 'US'

    def international(order):
        return not domestic(order)

    def batch1(order):
        return order.batch == 1

    def f2k(order):
        return order.batch == 'f2k'

    def processed(order):
        return order.number in ss_orders

    def from2016(order):
        return order.created_at.year == 2016

    # Read latest exports
    order_csv = latest_csv('order')
    user_csv  = latest_csv('user')
    users = User(user_csv).to_dict()
    orders = Order(order_csv, users).to_list()

    # Create several lists for accounting purposes
    open_orders      = filter(open, orders)
    cancelled_orders = filter(cancelled, orders)
    invalid_orders   = filter(invalid, orders)
    disputed_orders  = filter(disputed, orders)

    # Select orders we care about
    def predicates(order):
        return all([
            open(order),
            not cancelled(order),
            not disputed(order),
            not locked(order),
            not processed(order),
            domestic(order),
            batch1(order),
            # from2016(order),
            # f2k(order),
        ])

    selected_orders = filter(predicates, orders)

    totals = tuple(len(x) for x in (orders, open_orders, cancelled_orders,
                               invalid_orders, selected_orders))
    print 'orders total: %r, open: %r, cancelled: %r, invalid: %r, total: %r' % totals
    for order in invalid_orders:
        print order
    return selected_orders


def shipwire_login():
    return Shipwire(username='dev@hanzo.ai',
                    password='',
                    host='api.shipwire.com')


def submit_orders(orders):
    sw = shipwire_login()
    v = {
        'options': {
            'serviceLevelCode': 'GD',
        },
        'orderNo' : 'filter-shipwire-test',
        'externalId': '',
        'shipTo' : {
            'name' : 'Imran Hameed',
            'address1' : '33 N Almadden Blvd Unit 1601',
            'address2': '',
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

    print json.dumps(v, indent=4)

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


if __name__ == '__main__':
    # write_cached()
    orders = get_orders()
    # submit_orders(orders)
