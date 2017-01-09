#!/usr/bin/env python
import argparse
import csv
import sys
import json

csv.field_size_limit(sys.maxsize)

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('file', action='store', help='orders export from datastore')

    args = parser.parse_args()

    with open(args.file) as f:
        first_row = next(f).split(',')
        header = dict((k.strip(),i) for i,k in enumerate(first_row))

        items_idx  = header['Items_']
        status_idx = header['Status']
        test_idx   = header['Test']
        total_idx  = header['Total']
        paid_idx   = header['Paid']

        total_ordered  = 0
        total_refunded = 0

        for i, row in enumerate(csv.reader(f)):
            if int(row[total_idx]) == 50:
                continue
            if row[test_idx] == 'True':
                continue

            try:
                items  = json.loads(row[items_idx])
            except:
                print 'Failed to load item row', i
                continue

            status = row[status_idx]
            paid   = row[paid_idx]

            for item in items:
                if status == 'open':
                    total_ordered  += item['quantity']
                elif status == 'cancelled':
                    total_refunded += item['quantity']

        print 'Total open orders', total_ordered
        print 'Total orders refunded', total_refunded
