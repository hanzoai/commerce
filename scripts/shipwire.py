import json
import shipwire


def connect():
    return shipwire.Shipwire(username='dev@hanzo.ai',
                             password='',
                             host='api.shipwire.com')


class Shipwire(object):
    """
    Simple wrapper around Shipwire library.
    """

    def __init__(self):
        self.sw = connect()

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
                'email':      order.email,
                'name':       order.shipping_address_name,
                'address1':   order.shipping_address_line1,
                'address2':   order.shipping_address_line2,
                'city':       order.shipping_address_city,
                'state':      order.shipping_address_state,
                'country':    order.shipping_address_country,
                'postalCode': order.shipping_address_postal_code,
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


def read_cache():
    with open('shipwire.json') as f:
        for line in f:
            yield json.loads(line)


def write_cache():
    sw = connect()
    r  = sw.orders.list()

    with open('shipwire.json', 'w') as f:
        for order in r.all_serial():
            f.write(json.dumps(order['resource']) + '\n')
