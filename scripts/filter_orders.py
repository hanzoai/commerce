#!/usr/bin/env python
import os
from datetime import datetime
from export import Export, json, latest_csv, to_csv
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
    def __init__(self, filename, users, ss_orders):
        super(Order, self).__init__(filename)
        self.users     = users
        self.ss_orders = ss_orders

    fields = {
        'Id_':           str,
        'CreatedAt':     lambda x: datetime.strptime(x, '%Y-%m-%dT%H:%M:%S'),
        'Items_':        json,
        'Metadata_':     json,
        'Status':        lambda x: x.lower(),
        'PaymentStatus': lambda x: x.lower(),
        'Test':          bool,
        'UserId':        str,
        'Number':        str,
        'Total':         int,

        'ShippingAddress.Name':       str,
        'ShippingAddress.Country':    lambda x: x.upper(),
        'ShippingAddress.State':      lambda x: x.upper(),
        'ShippingAddress.City':       lambda x: x.upper(),
        'ShippingAddress.PostalCode': lambda x: x.upper(),
        'ShippingAddress.Line1':      lambda x: x.upper(),
        'ShippingAddress.Line2':      lambda x: x.upper(),

        # Virtual fields, hydrated or populated later
        'email':      None,
        'first_name': None,
        'last_name':  None,
        'batch':      None,

        'ss_status':      None,
        'ss_country':     None,
        'ss_state':       None,
        'ss_city':        None,
        'ss_postal_code': None,
        'ss_address1':    None,
        'ss_address2':    None,
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

        # Hydrate order with ss data
        ss_order = self.ss_orders.get(order.number, None)
        if ss_order:
            order.ss_status       = ss_order['status']
            order.ss_country      = ss_order['shipTo']['resource']['country']
            order.ss_state        = ss_order['shipTo']['resource']['state']
            order.ss_city         = ss_order['shipTo']['resource']['city']
            order.ss_postal_code  = ss_order['shipTo']['resource']['postalCode']
            order.ss_address1     = ss_order['shipTo']['resource']['address1']
            order.ss_address2     = ss_order['shipTo']['resource']['address2']

        return order


def get_orders():
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
        return order.ss_status and order.ss_status != 'cancelled'

    def from2016(order):
        return order.created_at.year == 2016

    # Read in shipwire orders
    ss_orders = dict((x['orderNo'], x) for x in read_cached())

    # Read latest exports
    order_csv = latest_csv('order')
    user_csv  = latest_csv('user')
    users = User(user_csv).to_dict()
    orders = Order(order_csv, users, ss_orders).to_list()

    # Create several lists for accounting purposes
    open_orders      = [x for x in orders if open(x)]
    cancelled_orders = [x for x in orders if cancelled(x)]
    invalid_orders   = [x for x in orders if invalid(x)]
    disputed_orders  = [x for x in orders if disputed(x)]

    # Filter for orders we care about
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
    filtered_orders = [x for x in orders if predicates(x)]

    # Print stats and flag any invalid orders
    print 'Order statistics'
    totals = tuple(len(x) for x in (orders, open_orders, cancelled_orders,
                               disputed_orders, invalid_orders,
                               filtered_orders))
    print '  Total: {}, Open: {}, Cancelled: {}, Disputed: {}, Invalid: {}, Filtered: {}'.format(*totals)

    if invalid_orders:
        print 'Found the following invalid orders:'
        for order in invalid_orders:
            print order

    return filtered_orders


if __name__ == '__main__':
    if not os.path.exists('shipwire.json'):
        print 'Fetching latest orders from Shipwire...'
        write_cached()
    else:
        print 'Using cached shipwire.json'
    orders = get_orders()
    to_csv(orders, 'selected_orders.csv')
