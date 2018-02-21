#!/usr/bin/env python2

import os
import sys

PLATFORM_PATH = os.path.abspath(os.path.join(os.path.dirname(__file__), '..'))
SDK_PATH      = os.path.join(PLATFORM_PATH, '.sdk')
SERVER_PATH   = os.path.join(SDK_PATH, 'dev_appserver.py')

# Rewrite default test ports to something we can predict
for i, a in enumerate(sys.argv):
    if a == '--admin_port=0':
        sys.argv[i] = '--admin_port=8000'
    if a == '--api_port=0':
        sys.argv[i] = '--api_port=8001'
    if a == '--port=0':
        sys.argv[i] = '--port=8002'

# Remove generated app.yaml from arg stack
sys.argv.pop()

sys.argv.extend([
    '--dev_appserver_log_level=debug',
    '--enable_task_running=true',
    os.path.join(PLATFORM_PATH, 'config/test/app.yaml'),
    os.path.join(PLATFORM_PATH, 'api/app.dev.yaml'),
])

argv = 'python2 {} {}'.format(SERVER_PATH, ' '.join(sys.argv[1:]))
print(argv)

sys.exit(os.system(argv))
