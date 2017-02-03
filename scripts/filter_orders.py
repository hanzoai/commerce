#!/usr/bin/env python
from datetime import datetime
from itertools import islice
import os

from export import Export, json, latest_csv, to_csv
from export.filter import *
import shipwire


class User(Export):
    fields = {
        'Id_':       str,
        'Email':     str,
        'FirstName': str,
        'LastName':  str,
    }


class Order(Export):
    def __init__(self, filename, users, s_orders):
        super(Order, self).__init__(filename)
        self.users    = users
        self.s_orders = s_orders

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

        # Hydrated by user
        'email':      None,
        'first_name': None,
        'last_name':  None,
        'batch':      None,


        # Hydrated by Shipwire
        's_status':      None,
        's_country':     None,
        's_state':       None,
        's_city':        None,
        's_postal_code': None,
        's_address1':    None,
        's_address2':    None,
    }

    def ignore(self, order):
        """Ignore test orders."""
        return order.test or order.total == 50

    def hydrate(self, order):
        """Hydrate order object using user and Shipwire data."""
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
        user             = self.users[order.user_id]
        order.email      = user.email
        order.first_name = user.first_name
        order.last_name  = user.last_name

        # Hydrate order with Shipwire data
        s_order = self.s_orders.get(order.number, None)
        if s_order:
            order.s_status      = s_order['status']
            order.s_country     = s_order['shipTo']['resource']['country']
            order.s_state       = s_order['shipTo']['resource']['state']
            order.s_city        = s_order['shipTo']['resource']['city']
            order.s_postal_code = s_order['shipTo']['resource']['postalCode']
            order.s_address1    = s_order['shipTo']['resource']['address1']
            order.s_address2    = s_order['shipTo']['resource']['address2']

        return order


def get_orders(filter):
    """Return orders matching some predicate(s)."""

    # Load Shipwire orders
    s_orders = {x['orderNo']: x for x in shipwire.read_cache()}

    # Load latest users, orders
    users  = User(latest_csv('user')).to_dict()
    orders = Order(latest_csv('order'), users, s_orders).to_list()

    # Calculate some stats
    open_orders      = sum(1 for x in orders if open(x))
    cancelled_orders = sum(1 for x in orders if cancelled(x))
    invalid_orders   = sum(1 for x in orders if invalid(x))
    disputed_orders  = sum(1 for x in orders if disputed(x))

    # Filter orders
    selected_orders  = [x for x in orders if filter(x)]

    # Print stats
    print 'Order statistics'
    totals = (len(orders), open_orders, cancelled_orders, disputed_orders,
              invalid_orders, len(selected_orders))
    print '  Total: {}, Open: {}, Cancelled: {}, Disputed: {}, Invalid: {}, selected: {}'.format(*totals)

    # Print any invalid orders
    if invalid_orders:
        print 'Found the following invalid orders:'
        for order in orders:
            if invalid(order):
                print order

    return selected_orders


if __name__ == '__main__':
    # Fetch Shipwire db if needed
    if not os.path.exists('shipwire.json'):
        print 'Fetching latest orders from Shipwire...'
        shipwire.write_cache()
    else:
        print 'Using cached shipwire.json'

    # Filter orders
    orders = get_orders(lambda order: all((
        open(order),
        not cancelled(order),
        not disputed(order),
        not locked(order),
        not processed(order),
        domestic(order),
        batch1(order),
        # from2016(order),
        # f2k(order),
    )))

    # Sort by value
    orders.sort(key=lambda x: x.total, reverse=True)

    # Top 10
    orders = islice(orders, 10)

    # Write orders to CSV
    to_csv(orders, 'orders.csv')
