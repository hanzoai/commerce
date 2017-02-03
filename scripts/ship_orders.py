#!/usr/bin/env python
from export import Export, json, latest_csv, to_csv
from datetime import datetime
from shipwire import *

class Order(Export):
    fields = {
        'Id_':           str,
        'Items_':        json,
        'Number':        str,
        'Email':         str,

        'ShippingAddress.Name':       str,
        'ShippingAddress.Country':    str,
        'ShippingAddress.State':      str,
        'ShippingAddress.City':       str,
        'ShippingAddress.PostalCode': str,
        'ShippingAddress.Line1':      str,
        'ShippingAddress.Line2':      str,
    }

class Shipwire(object):
    def __init__(self):
        self.client = Shipwire(username='dev@hanzo.ai',
                               password='',
                               host='api.shipwire.com')

    def submit_order(self, order):
        # Only handle earphones for now
        sku      = '686696998137'
        quantity = 0
        for item in order.items_:
            if item.productId == 'wycZ3j0kFP0JBv':
                quantity += item.quantity

        payload = {
            'options': {
                'serviceLevelCode': 'GD',
            },
            'orderNo': order.number,
            'externalId': order.id_,
            'shipTo' : {
                'email':     order.email,
                'name':      order.shipping_address_name,
                'address1':  order.shipping_address_line1,
                'address2':  order.shipping_address_line2,
                'city':      order.shipping_address_city,
                'state':     order.shipping_address_state,
                'country':   order.shipping_address_country,
            },
            'items' : [
                {
                    'sku':      sku,
                    'quantity': quantity,
                }
            ],
        }

        print json.dumps(payload, indent=4)

        res = sw.order.create(json=payload)

        print '######### BEGIN'
        print '######### res.status'
        pprint(res.status)
        print '######### res.message'
        pprint(res.message)
        print '######### res.json'
        pprint(res.json)
        print '######### res.location'
        pprint(res.location)
        print '######### res.warnings'
        pprint(res.warnings)
        print '######### res.errors'
        pprint(res.errors)
        print '######### END'

if __name__ == '__main__':
    orders = Order('filtered_orders.csv').to_list()
    sw = Shipwire()
    for order in order:
        sw.submit_order(order)
