#!/usr/bin/env python
from export import Export, json, latest_csv, to_csv


class User(Export):
    fields = {
        'Id_': str,
        'FirstName': str,
        'LastName': str,
        'Email': str,
    }


class Order(Export):
    fields = {
        # Fields in CSV
        'Id_': str,
        'Items_': json,
        'Metadata_': str,
        'Status': str,
        'Test': bool,
        'UserId': str,
        'Total': int,
        'ShippingAddress.Country': lambda x: x.lower(),

        # Not in CSV, populated from user
        'Email': None,
        'FirstName': None,
        'LastName': None,
        'Batch': None,
    }


if __name__ == '__main__':
    # Find latest exports
    order_csv = latest_csv('order')
    user_csv  = latest_csv('user')

    # Read orders and organize by user id for easy reference
    orders = Order(order_csv).to_list()
    users = User(user_csv).to_dict()

    customers = []
    for order in orders:
        # Skip test orders
        if order.test:
            continue
        if order.total == 50:
            continue

        # Skip cancelled order
        if order.status != 'open':
            continue

        # Skip international
        if order.shipping_address_country != 'us':
            continue

        # Get batch number
        order.batch = 1
        if order.metadata_ == '{"batch":"2"}':
            order.batch = 2

        # Skip batch 2
        if order.batch == 2:
            continue

        # Hydrate order with user data
        user = users.get(order.user_id, None)
        if user:
            order.email      = user.email
            order.first_name = user.first_name
            order.last_name  = user.last_name
            customers.append(order)

    # Save customer data
    to_csv(customers, 'customers.csv')
