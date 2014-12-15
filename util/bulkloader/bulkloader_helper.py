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

def generate_preorder(value, bulkload_state):
    """
    Generates preorder information from Items.SKU_ and Items.Quantity.
    """
    row = bulkload_state.current_dictionary
    if 'Items.SKU' not in row or 'Items.Quantity' not in row:
        return ''

    skus = row['Items.SKU']
    qtys = row['Items.Quantity']

    if not skus or not qtys:
        return ''

    # Regenerate lists from transformed Items.SKU, Items.Quantity
    skus = [sku.strip() for sku in skus.split(',') if sku]
    qtys = [qty.strip() for qty in qtys.split(',') if qty]

    return ', '.join(['%s: %s' % order for order in zip(skus, qtys)])


def join_address_lines(value, bulkload_state):
    """
    Generates preorder information from Items.SKU_ and Items.Quantity.
    """
    row = bulkload_state.current_dictionary
    if 'ShippingAddress.Line1' not in row or 'ShippingAddress.Line2' not in row:
        return ''

    line1 = row['ShippingAddress.Line1'] or ''
    line2 = row['ShippingAddress.Line2'] or ''

    return (line1 + ' ' + line2).strip()


def na_if_none(value):
    """
    Return N/A instead of empty string.
    """
    if not value:
        return 'N/A'
    return value
