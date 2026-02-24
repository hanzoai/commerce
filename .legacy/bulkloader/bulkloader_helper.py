import datetime
import base64


def list_num_to_str(value):
    """
    Convert a list of ints into comma-joined str.
    """
    if isinstance(value, list):
        return ', '.join(map(str, value))
    if value:
        return value
    return ''


def list_to_str(value):
    """
    Convert a list of strs into comma-joined str.
    """
    if isinstance(value, list):
        return ', '.join(value)
    if value:
        return value
    return ''


def list_skus_to_str(value):
    """
    Strip `SKULLY-` prefix from SKUs.
    """
    return list_to_str(value).replace('SKULLY-', '')


def na_if_none(value):
    """Return N/A instead of empty string."""
    if not value:
        return 'N/A'
    return value


def empty_string_if_none(value):
    """Return '' instead of None."""
    if not value:
        return ''
    return value


def generate_preorder_description(value, bulkload_state):
    """Generates preorder information from Items.SKU_ and Items.Quantity."""
    row = bulkload_state.current_dictionary

    if 'Items.SKU_' not in row or 'Items.Quantity' not in row:
        return ''

    skus = eval(row['Items.SKU_'] or 'None')
    qtys = eval(row['Items.Quantity'] or 'None')

    if not skus or not qtys:
        return ''

    return ', '.join(['%s: %s' % order for order in zip(skus, qtys)])


def join_address_lines(value, bulkload_state):
    """Join address lines to create a single address line."""
    row = bulkload_state.current_dictionary
    if 'ShippingAddress.Line1' not in row or 'ShippingAddress.Line2' not in row:
        return u''

    line1 = row.get('ShippingAddress.Line1', u'')
    line2 = row.get('ShippingAddress.Line2', u'')

    return u' '.join([line1, line2]).strip()


def generate_shipping_address(value, bulkload_state):
    """Generates shipping address."""
    row = bulkload_state.current_dictionary
    address = []
    keys = {
        'line1':       'ShippingAddress.Lines',
        'line2':       'ShippingAddress.Line2',
        'city':        'ShippingAddress.City',
        'state':       'ShippingAddress.State',
        'postal_code': 'ShippingAddress.PostalCode',
        'country':     'ShippingAddress.Country',
    }

    get = lambda key: row.get(keys[key], u'')

    if keys['line1'] not in row and keys['line2'] not in row:
        return u''

    # Add Street address
    address.append(u'{0} {1}'.format(get('line1'), get('line2')))
    # Add City, State, zip line
    address.append(
        u'{0}, {1} {2}'.format(
            get('city'),
            get('state'),
            get('postal_code')))
    # Add country
    address.append(get('country'))
    return '\n'.join([line.strip() for line in address if line])


def import_date_time(format, _strptime=None):
    """
    A wrapper around strptime. Also returns None if the input is empty.

    Args:
      format: Format string for strptime.

    Returns:
      Single argument method which parses a string into a datetime using format.
    """

    if not _strptime:
        _strptime = datetime.datetime.strptime

    def import_date_time_lambda(value):
        if not value:
            return None
        return _strptime(value, format)

    return import_date_time_lambda


def export_date_time(format):
    """
    A wrapper around strftime. Also returns '' if the input is None.

    Args:
      format: Format string for strftime.

    Returns:
      Single argument method which convers a datetime into a string using format.
    """

    def export_date_time_lambda(value):
        if not value:
            return '2014-08-10 00:00:01'
        try:
            return datetime.datetime.strftime(value, format)
        except:
            return '2014-08-10 00:00:01'

    return export_date_time_lambda


def transform_eval(value):
    """Coerces a stringified list of ints into a list of ints."""
    if value == '' or value is None or value == []:
        return None
    return eval(value)

def base64_encode_else_none(value):
    """Base64 encodes value if not none."""
    if value:
        return base64.b64encode(value)
    else:
        return ''
