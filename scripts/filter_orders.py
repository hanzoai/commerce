#!/usr/bin/env python
import os
from datetime import datetime
from export import Export, json, latest_csv, to_csv
from shipwire import *
from shipwire_export import read_cached, write_cached


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


        # Hydrated by shipwire
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
        """Hydrate order object using user and shipwire data."""
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

        # Hydrate order with shipwire data
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


def get_orders():
    """Return orders matching some predicate(s)."""

    # Various predicates to use for filtering orders
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
        return order.s_status and order.s_status != 'cancelled'

    def from2016(order):
        return order.created_at.year == 2016

    # Load shipwire orders
    s_orders = dict((x['orderNo'], x) for x in read_cached())

    # Load latest users, orders
    users  = User(latest_csv('user')).to_dict()
    orders = Order(latest_csv('order'), users, s_orders).to_list()

    # Calculate some stats
    open_orders      = len([x for x in orders if open(x)])
    cancelled_orders = len([x for x in orders if cancelled(x)])
    invalid_orders   = len([x for x in orders if invalid(x)])
    disputed_orders  = len([x for x in orders if disputed(x)])

    # Filter for orders we care about
    def predicates(order):
        """
        Predicates to check before selecting order to ship. For example, to
        ship a single order:

        if order.id_ == 'qnirQmhvHO4q0IrG947':
            return True
        else:
            return False
        """

        return all((
            open(order),
            not cancelled(order),
            not disputed(order),
            not locked(order),
            not processed(order),
            domestic(order),
            batch1(order),
            # from2016(order),
            # f2k(order),
        ))

    filtered_orders = [x for x in orders if predicates(x)]

    # Print stats and flag any invalid orders
    print 'Order statistics'
    totals = (len(orders), open_orders, cancelled_orders, disputed_orders,
              invalid_orders, len(filtered_orders))
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
    to_csv(orders, 'filtered_orders.csv')
