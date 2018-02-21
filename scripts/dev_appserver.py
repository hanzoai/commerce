#!/usr/bin/env python2

import os
import sys


PLATFORM_PATH = os.path.abspath(os.path.join(os.path.dirname(__file__), '..'))
SDK_PATH      = os.path.join(PLATFORM_PATH, '.sdk')
SERVER_PATH   = os.path.join(SDK_PATH, 'dev_appserver.py')

PORTS = {
    '--admin_port=0': int(os.environ['DEV_APPSERVER_ADMIN_PORT']),
    '--api_port=0':   int(os.environ['DEV_APPSERVER_API_PORT']),
    '--port=0':       int(os.environ['DEV_APPSERVER_PORT']),
}

if __name__ == '__main__':
    # Update dev_appserver ports with randomized port numbers
    for i, argv in enumerate(sys.argv):
        if argv in PORTS:
            sys.argv[i] = argv.replace('0', str(PORTS[argv]))

    # Remove generated app.yaml from arg stack
    sys.argv.pop()

    sys.argv.extend([
        # '--dev_appserver_log_level=info',
        '--enable_task_running=true',
        os.path.join(PLATFORM_PATH, 'config/test'),
        os.path.join(PLATFORM_PATH, 'api/app.dev.yaml'),
    ])

    argv = 'python2 {} {}'.format(SERVER_PATH, ' '.join(sys.argv[1:]))
    print(argv)

    sys.exit(os.system(argv))
