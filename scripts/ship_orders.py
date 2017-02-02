#!/usr/bin/env python
from export import Export, json, latest_csv, to_csv
from shipwire import *


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
        'Items_': json,
        'Metadata_': str,
        'Status': lambda x: x.lower(),
        'PaymentStatus': lambda x: x.lower(),
        'Test': bool,
        'UserId': str,
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
        # Add batch number
        order.batch = 1
        if order.metadata_ == '{"batch":"2"}':
            order.batch = 2

        # Add info from users
        user = self.users[order.user_id]

        # Hydrate order with user data
        order.email      = user.email
        order.first_name = user.first_name
        order.last_name  = user.last_name
        return order

def get_orders():
    def open_order(order):
        return order.status == 'open' and order.payment_status == 'paid'

    def cancelled_order(order):
        return order.status == 'cancelled' or order.payment_status == 'refunded'

    def domestic(order):
        return order.shipping_address_country != 'us'

    def batch1(order):
        return order.batch == 1

    # Read latest exports
    order_csv = latest_csv('order')
    user_csv  = latest_csv('user')
    users = User(user_csv).to_dict()
    orders = Order(order_csv, users).to_list()

    # Create several lists for accounting purposes
    open_orders      = filter(open_order, orders)
    cancelled_orders = filter(cancelled_order, orders)
    invalid_orders   = filter(lambda order: not open_order(order) and not cancelled_order(order), orders)

    # Select orders we care about
    def selection(order):
        # Filter orders based on what we care about
        if not open_order(order):
            return False
        if cancelled_order(order):
            return False
        if not batch1(order):
            return False
        if not domestic(order):
            return False
        return True

    selected_orders = filter(selection, orders)

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


def submit_orders():
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
    orders = get_orders()
    # sw = shipwire_login()
    # submit_test_order(sw, None)
