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
        'UserId': str,
        'Metadata_': str,
        'Test': bool,
        'Items_': json,

        # Not in CSV, populated from user
        'Email': None,
        'FirstName': None,
        'LastName': None,
    }


if __name__ == '__main__':
    # Find latest exports
    order_csv = latest_csv('order')
    user_csv  = latest_csv('user')

    # Read orders and organize by user id for easy reference
    orders = Order(order_csv).to_dict()
    users = User(user_csv).to_dict()

    # Hydrate order with user data
    customers = []
    for order in orders.values():
        user = users.get(order.user_id, None)
        if user:
            order.email      = user.email
            order.first_name = user.first_name
            order.last_name  = user.last_name
            customers.append(order)

    # Save customer data
    to_csv(customers, 'customers.csv')
