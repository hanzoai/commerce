import requests
import json
from requests.auth import HTTPBasicAuth


API_ENDPOINT = 'https://stoned.reamaze.com/api/v1/'
API_USER     = 'dev@hanzo.ai'
API_TOKEN    = ''

def get(url, page=None):
    auth = HTTPBasicAuth(API_USER, API_TOKEN)
    headers = {'Accept': 'application/json'}

    url = API_ENDPOINT + url
    if page:
        url = url + '?page=' + str(page)

    res = requests.get(url=url, auth=auth, headers=headers)
    return res.json()

def get_contacts(page=None):
    return get('contacts', page=page)

if __name__ == '__main__':
    first    = get_contacts()
    contacts = first['contacts']
    pages    = first['page_count']

    for page in range(2, pages+1):
        contacts += get_contacts(page=page)['contacts']

    with open('reamaze_export.json', 'w') as f:
        for contact in contacts:
            f.write(json.dumps(contact) + '\n')
