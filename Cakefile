exec    = require('executive').interactive
fs      = require 'fs'
{spawn} = require 'child_process'

instances = [
  '23.21.131.233'  # kaching-us-east-1b (i-e4565e9f)
]

option '-e', '--email [email]', 'email of user'
option '-i', '--id [id]', 'id of extension'
option '-h', '--host [host]', 'host address to bind to'
option '-p', '--port [port]', 'port to run use'

identityFile = process.env.HOME + '/.ssh/verus.pem'
sshOpts = '-o LogLevel=quiet -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i ' + identityFile

# run a command or array of commands on an instance or array of instances
run = (cmds, hosts, callback = ->) ->
  if Array.isArray cmds
    cmds = cmds.join ';'

  unless Array.isArray hosts
    hosts = [hosts]

  jobs = []
  for host in hosts
    jobs.push "echo Connecting to #{host}"
    jobs.push "ssh #{sshOpts} ubuntu@#{host} \"#{cmds}\""

  exec jobs, callback

question = (question, callback) ->
  rl = require('readline').createInterface
    input: process.stdin
    output: process.stdout
  rl.question question, (answer) ->
    rl.close()
    callback answer

task 'run', 'run development server', (opts) ->
  require('rocksteady').run './app',
    host: opts.host ? 'localhost'
    port: opts.port ? 3000

task 'run:local', 'bind to local host and use port 80 to emulate production', ->
  require('executive').quiet 'ifconfig', (err, stdout) ->
    lines = stdout.split '\n'
    for line in lines
      if match = /inet (192\.\d{1,3}\.\d{1,3}\.\d{1,3})/.exec line
        hostname = match[1]
        exec "sudo cake -p 80 -h #{hostname} run"
        return

    console.log "couldn't figure out your hostname"

task 'createdb', 'create datbase', ->
  db = require './db'
  db.createdb ->
    db.end()

task 'dropdb', 'drop database', ->
  db = require './db'
  db.dropdb ->
    db.end()

task 'remove-purchase', (options) ->
  db = require './db'
  db.removePurchase options.email, options.id, ->
    db.end()

task 'test', 'run tests', (options) ->
  console.log 'todo'

task 'deploy', 'deploy code to servers', (options) ->
  branch = options.branch ? 'master'

  cmds = """
  cd /var/apps/kaching
  git fetch origin
  git checkout #{branch}
  git branch | grep #{branch} && git reset --hard origin/#{branch}
  npm install --production
  sudo /etc/init.d/nginx restart
  sudo reload kaching
  """.split '\n'

  if options.pretend?
    return console.log cmds

  if options.host?
    run cmds, options.host
  else if options.instance?
    run cmds, instances[options.instance]
  else
    question 'Are you sure you want to deploy to production? (y/n) ', (answer) ->
      if answer.charAt(0).toLowerCase() == 'y'
        run cmds, instances

task 'install', 'install kaching on fresh server', (options) ->
  branch = options.branch ? 'master'
  cmds = """
  sudo apt-get -y install nginx
  cd /var/apps
  rm -rf kaching
  git clone ssh://git@github.com/DMagic/kaching
  cd kaching
  git checkout #{branch}
  npm install --production
  cd /etc/init
  sudo rm -f kaching.conf
  sudo ln -s /var/apps/kaching/conf/upstart.conf kaching.conf
  sudo initctl reload-configuration
  cd /etc/nginx/sites-enabled
  sudo rm -f kaching.conf
  sudo ln -s /var/apps/kaching/conf/nginx.conf kaching.conf
  sudo stop kaching &> /dev/null
  sudo start kaching
  sudo /etc/init.d/nginx restart
  """.split '\n'

  if options.pretend?
    return console.log cmds

  if options.host?
    run cmds, options.host
  else if options.instance?
    run cmds, instances[options.instance]
  else
    question 'This is a potentially destructive operation, continue? (y/n) ', (answer) ->
      if answer.charAt(0).toLowerCase() == 'y'
        run cmds, instances

task 'exec', 'ssh into instance and execute a command', (options) ->
  unless options.exec?
    return console.log 'please specifiy a command to run with --exec'

  if options.pretend?
    return console.log options.exec

  if options.host?
    run options.exec, options.host
  else if options.instance?
    run options.exec, instances[options.instance]
  else
    run options.exec, instances

task 'tail', 'tail /var/log/kaching', (options) ->
  options.exec = 'tail -f /var/log/kaching.log'
  invoke 'exec'

task 'ssh', 'ssh into ec2 instance', (options) ->
  host = options.host ? instances[options.instance] ? instances[0]
  args = [
    "ubuntu@#{host}"
    '-i'
    identityFile
  ]

  if options.pretend?
    return console.log cmd

  spawn 'ssh', args, stdio: [0, 1, 2]

task 'update:system', 'update software on instance', (options) ->
  options.exec = '''
  sudo apt-get update
  sudo apt-get -y upgrade
  '''.split '\n'

  invoke 'exec'

task 'update:maxmind', 'update maxmind database on instance', (options) ->
  options.exec = '''
  sudo chmod 777 /usr/share/GeoIP
  cd /usr/share/GeoIP
  echo downloading latest MaxMind databases
  sudo curl 'http://download.maxmind.com/app/geoip_download?edition_id=106&suffix=tar.gz&license_key=NgFEzguN66jZ' > country.tar.gz
  sudo curl 'http://download.maxmind.com/app/geoip_download?edition_id=132&suffix=tar.gz&license_key=NgFEzguN66jZ' > city.tar.gz
  echo extracting...
  sudo tar xvf country.tar.gz --wildcards --no-anchored '*.dat' --transform='s/.*/GeoIP.dat/'
  sudo tar xvf city.tar.gz --wildcards --no-anchored '*.dat' --transform='s/.*/GeoIPCity.dat/'
  sudo /etc/init.d/nginx reload
  '''.split '\n'

  invoke 'exec'

task 'update:dotfiles', 'update dotfiles on instance', (options) ->
  options.exec = '~/.ellipsis/bin/ellipsis update'
  invoke 'exec'

task 'build:static', 'compile static files', ->
  require('requisite').bundle
    entry:   __dirname + '/assets/js/app'
  , (err, bundle) ->
    fs.writeFileSync 'app.js', bundle.toString()

task 'sql', 'connect to kaching database', (options) ->
  config = require('./settings').db

  args = [
    '-h', config.host
    '--user='+ config.user
    '--password='+ config.password
    '-D', config.database
    '--table'
  ]

  if options.pretend?
    return console.log 'mysql', args.join ' '

  spawn 'mysql', args, stdio: [0, 1, 2]
