import csv
import glob
import json
import os
import re
import sys
from recordtype import recordtype

csv.field_size_limit(sys.maxsize)


def snake(name):
    """
    Converts a Go-style name into the Python equivalent.
    """
    s1 = re.sub('(.)([A-Z][a-z]+)', r'\1_\2', name)
    s2 = re.sub('([a-z0-9])([A-Z])', r'\1_\2', s1).lower()
    s3 = s2.replace('.', '')
    return s3


def from_bool(b):
    """
    Deserialize boolean column into Python bool.
    """
    if b == 'True':
        return True
    elif b == 'False':
        return False
    else:
        raise Exception('Invalid boolean: {0}'.format(b))


def from_json(obj):
    """
    Deserialize JSON column into appropriate Python object.
    """
    return json.loads(obj)


class Parser(object):
    """
    Parses CSV row into Python objects with specified attributes, transforming
    columns as necessary.
    """

    def __init__(self, constructor, header, fields):
        self.constructor = constructor
        self.header      = header
        self.fields      = fields

    def get_column(self, row, field):
        idx = self.header[field]
        return row[idx]

    def parse(self, row):
        values = []

        for field, transform in self.fields.items():
            # Skip virtual columns
            if transform == None:
                values.append(None)
                continue

            # Get value for this column
            val = self.get_column(row, field)

            # Apply any necessary transformations
            if transform == str:
                values.append(val)
            elif transform == bool:
                values.append(from_bool(val))
            elif transform == json:
                values.append(from_json(val))
            else:
                values.append(transform(val))

        return self.constructor(*values)


class Export(object):
    """
    Wrapper around on-disk CSV exports which allows them to easily be
    transformed into normal Python objects.
    """
    fields = {}

    def __init__(self, filename):
        class_name = self.__class__.__name__

        self.filename = filename
        self.header   = self.parse_header()
        self.constructor    = recordtype(class_name, [snake(f) for f in self.fields])

    def parse_header(self):
        """
        Create map for CSV colum layout based on header.
        """
        with open(self.filename) as f:
            first_row = next(f).split(',')
            return dict((k.strip(),i) for i,k in enumerate(first_row))

    def get_parser(self):
        """
        Get parser for this export.
        """
        return Parser(self.constructor, self.header, self.fields)

    def ignore(self, obj):
        """
        Ignore an arbitrary object.
        """
        return False

    def read_csv(self):
        """
        Lazily read CSV converting rows into records.
        """
        with open(self.filename) as f:
            next(f)  # Skip header
            parser = self.get_parser()
            for row in csv.reader(f):
                obj = parser.parse(row)
                if not self.ignore(obj):
                    yield obj

    def to_list(self):
        """
        Convert export into list of records.
        """
        return list(self.read_csv())

    def to_dict(self, key='id_'):
        """
        Convert export into dict of records using some key.
        """
        d = {}
        for obj in self.read_csv():
            k = getattr(obj, key)
            d[k] = obj
        return d


def guess_fields(obj):
    """
    Guess fields expected to export based on public attributes of a Python
    object.
    """
    fields = []
    for attr in dir(obj):
        if attr.startswith('_'):
            continue

        if not callable(getattr(obj, attr)):
            fields.append(attr)

    return fields


def to_csv(entities, filename, fields=None):
    """
    Write list of entities into CSV.
    """

    if not fields:
        fields = guess_fields(entities[0])

    with open(filename, 'w') as f:
        writer = csv.writer(f)
        writer.writerow(fields)
        for entity in entities:
            writer.writerow([getattr(entity, field) for field in fields])


def latest_csv(kind):
    """
    Find latest export CSV for a given kind.
    """
    files = filter(os.path.isfile, glob.glob('_export/*.csv'))
    files.sort(key=lambda x: os.path.getmtime(x), reverse=True)
    for fn in files:
        if fn.split('-')[1] == kind.lower():
            return fn
    return None
