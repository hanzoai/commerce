import re
import csv
import json
import os
import glob
from datetime import datetime


def snake(name):
    """Converts a Go-style name into the Python equivalent."""
    s1 = re.sub('(.)([A-Z][a-z]+)', r'\1_\2', name)
    s2 = re.sub('([a-z0-9])([A-Z])', r'\1_\2', s1).lower()
    s3 = s2.replace('.', '')
    return s3


def guess_fields(obj):
    """Guess fields expected to export based on public attributes of a Python object."""
    fields = []
    for attr in dir(obj):
        if attr.startswith('_'):
            continue

        if not callable(getattr(obj, attr)):
            fields.append(attr)

    return fields


def to_json(obj):
    """Serialize object to JSON, but do not quote strings."""
    def serializer(obj):
        if isinstance(obj, datetime):
            return obj.isoformat()
        raise TypeError("Type not serializable")
    s = json.dumps(obj, default=serializer)
    return re.sub(r'^"|"$', '', s)


def to_csv(entities, filename, fields=None):
    """Write list of entities into CSV."""

    if not fields:
        fields = guess_fields(entities[0])

    with open(filename, 'w') as f:
        writer = csv.writer(f)
        writer.writerow(fields)
        for entity in entities:
            values = [getattr(entity, field) for field in fields]
            writer.writerow([to_json(s) for s in values])


def latest_csv(kind):
    """Find latest export CSV for a given kind."""
    files = filter(os.path.isfile, glob.glob('_export/*.csv'))
    files.sort(key=lambda x: os.path.getmtime(x), reverse=True)
    for fn in files:
        if fn.split('-')[1] == kind.lower():
            return fn
    return None
