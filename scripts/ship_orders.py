#!/usr/bin/env python
from export import Export, json, latest_csv, to_csv
from datetime import datetime
import shipwire


class Order(Export):
    fields = {
        'id_':    str,
        'items_': json,
        'number': str,
        'email':  str,

        'shipping_address_name':        str,
        'shipping_address_country':     str,
        'shipping_address_state':       str,
        'shipping_address_city':        str,
        'shipping_address_postal_code': str,
        'shipping_address_line1':       str,
        'shipping_address_line2':       str,
    }


class Shipwire(object):
    """
    Simple wrapper around Shipwire library.
    """

    def __init__(self):
        self.sw = shipwire.Shipwire(username='dev@hanzo.ai',
                                    password='',
                                    host='api.shipwire.com')

    def submit_order(self, order):
        # Only handle earphones for now
        sku = '686696998137'
        quantity = 0
        for item in order.items_:
            if item['productId'] == 'wycZ3j0kFP0JBv':
                quantity += item['quantity']

        payload = {
            'orderNo':    order.number,
            'externalId': order.id_,
            'options': {
                'serviceLevelCode': 'GD',
            },
            'shipTo': {
                'email':    order.email,
                'name':     order.shipping_address_name,
                'address1': order.shipping_address_line1,
                'address2': order.shipping_address_line2,
                'city':     order.shipping_address_city,
                'state':    order.shipping_address_state,
                'country':  order.shipping_address_country,
            },
            'items': [
                {
                    'sku':      sku,
                    'quantity': quantity,
                }
            ],
        }

        print json.dumps(payload, indent=4)

        res = self.sw.order.create(json=payload)

        print '######### BEGIN'
        print res.status, res.message
        print '######### res.json'
        print json.dumps(res.json, indent=4)
        print '######### res.location'
        print res.location
        print '######### res.warnings'
        print res.warnings
        print '######### res.errors'
        print res.errors
        print '######### END'


if __name__ == '__main__':
    sw     = Shipwire()
    orders = Order('filtered_orders.csv').to_list()

    for order in orders:
        sw.submit_order(order)
