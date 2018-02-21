#!/usr/bin/env python2

import os
import sys


PLATFORM_PATH = os.path.abspath(os.path.join(os.path.dirname(__file__), '..'))
SDK_PATH      = os.path.join(PLATFORM_PATH, '.sdk')
SERVER_PATH   = os.path.join(SDK_PATH, 'dev_appserver.py')


PORTS = {
    '--admin_port=0': int(os.environ['DEV_APP_SERVER_ADMIN_PORT']),
    '--api_port=0':   int(os.environ['DEV_APP_SERVER_API_PORT']),
    '--port=0':       int(os.environ['DEV_APP_SERVER_PORT']),
}


# dev_appserver will increment port number as services launch from port value.
# To avoid collisions we make sure it has the highest available port number.
def reassign_ports():
    # get highest pair
    highest = max(PORTS.items(), key=lambda x: x[1])
    current = PORTS['--port=0']
    PORTS['--port=0'] = highest[1]
    PORTS[highest[0]] = current


if __name__ == '__main__':
    reassign_ports()

    # Update dev_appserver ports with randomized port numbers
    for i, argv in enumerate(sys.argv):
        if argv in PORTS:
            sys.argv[i] = argv.replace('0', str(PORTS[argv]))

    # Remove generated app.yaml from arg stack
    sys.argv.pop()

    sys.argv.extend([
        '--dev_appserver_log_level=info',
        '--enable_task_running=true',
        os.path.join(PLATFORM_PATH, 'config/test/app.yaml'),
        os.path.join(PLATFORM_PATH, 'api/app.dev.yaml'),
    ])

    argv = 'python2 {} {}'.format(SERVER_PATH, ' '.join(sys.argv[1:]))
    print(argv)

    sys.exit(os.system(argv))
