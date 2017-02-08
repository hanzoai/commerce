# crowdstart [![Circle CI](https://circleci.com/gh/verus-io/crowdstart.svg?style=svg&circle-token=fbc175690392a3aa50b991100261397e56e8f29d)](https://circleci.com/gh/verus-io/crowdstart)
Crowdfunding platform.

## Development
You can use `make` to setup your development enviroment. Running:

```
$ make deps
```

...will download the Go App Engine SDK and unzip it into `.sdk/`. When hacking
on things you'll want to ensure `$GOROOT` and `$GOPATH` point to their
respective directories inside `.sdk/`.

You can source the provided `.env` file to set these variables, or
[`autoenv`](https://github.com/kennethreitz/autoenv) to set them automatically
when entering the project directory.

You can install the common Go command line tools and configure `gocode` to work
with App Engine by running:

```bash
$ make tools
```

You can then use `make serve` to run the local development server and `make
test` to run tests.

You can create a local `config.json` file containing configuration variables to
customize settings locally (for instance to disable the auto fixture loading).

## Deployment
We use [codeship](http://codeship.io) for continuous integration and deployment.
Pushing to master will (in the case of a successful test run) deploy the project
to production automatically.

## Architecture
- Go is used for all backend code.
- Datastore is primary database
- Hosted on Google App Engine

Crowdstart is broken up into several different [App Engine
Modules](https://cloud.google.com/appengine/docs/go/modules/), for clarity,
performance, reliability and scalability reasons. They are:

### `default` (`app.yaml`)
Default module, static file serving and App Engine warmup.

### `api` (`api/app.yaml`)
Implements backend models and provides abstraction layer between Google's
Datastore. `api/models` package is shared across project.

### `checkout` (`checkout/app.yaml`)
Implements secure checkout, generally hosted at `https://secure.hanzo.io`.
should be possible to CNAME to it from `secure.client.com` for branding
purposes.

### `store` (`store/app.yaml`)
This is a custom store for boutique luxury brands that need their own hosted
store/cart experience. This is versioned separately from our own internal
platform store, so that the client's designers can have a stable API to work
with. Implements store frontend, product pages, cart view, order management.

### `platform` (`platform/app.yaml`)
Our crowdfunding platform ala `backerkit.com` or `indiegogo.com`. Has our
"crowdstart" branded experience. This is the non-white label version of the
platform. Needs administrative interface similar to Indiegogo, with
reporting/stats/etc.

### `dispatch.yaml`
We use `dispatch.yaml` to pipe requests to the proper module. Unfortunately
configuring modules to route to different hostnames is not supported during
local development, which is why each module's handlers are set to a different
subdirectory.

#### Gotchas
- Routing is not relative, it's absolute.
- Order matters, first matching pattern takes precedence.
- Subdomains are incompatible with the local development server.
- Routing to the same url in multiple Go modules is not allowed (at least
  locally).
- All `goapp` incantations change once you use modules (see our
  [`Makefile`](Makefile) for details).
- If you update an `app.yaml`'s url patterns, make sure to update
  `dispatch.yaml` or they will be ignored.
- You have to run `appcfg.py update_dispatch` for dispatch rules to be used in
  production.

## Frontend UI

### store.hanzo.io (`store module`)
- Need to display hover when something is in cart with link to show cart page.

### / product listing
- product
    - thumbnail, title

### /product/<slug>
- title
- images
- description
- add to cart
- dropdowns
    - color
    - size
- force color/size stuff to be selected

### /show-cart
- products + total
- checkout

### /account
- Show orders
- Account information

### /create-account
### /login
### /logout
### /reset-password

### /orders/<order-id>
- Show order
- Current status
- Tracking info?
- Ability to manage order up until shipped

### secure.hanzo.io (`checkout` module)
- Requires SSL.

### /checkout/<cartid>
- billing info
- order summary
- shipping options
- continue
- display errors if unable to direct to complete
- save email / password for login?

### /checkout/complete
    - thank you

## Backend API
## api.hanzo.io (`api` module)
### /api/cart
- create, get, add, remove stuff from a cart

## Platform UI
### hanzo.io (`platform` module)

### /login
### /logout

## Models
Part of `api` module. See [`api/models`](api/models/models.go) package.

## TODO
- Support [multitenancy](https://cloud.google.com/appengine/docs/go/multitenancy/#Go_About_multitenancy).

## Crowdfunding platform architecture
```
Cycliq               Crowdstart							Why use an organization?  Why user a user?
Organization		    Users (Zach)                        - (API, Hosted store)	  - Just want a campaign
     |                    |                              - Your own data			  - Public on crowdstart
     v				     v
Product, Collection  Product, Collection, etc
     |				     |
     v                    v
Campaign (optional)  Campaign (page, default)
     |					 |
     v					 v
Order (org)		    Order (default)

Convert a campaign -> Organization
1. Create organization
2. Add org to user (which is an account, technically)
3. Move orders -> organization
4. Clone user -> organization (might go out of sync but who cares), set cid to point back to original
5. List new order id on original user

On public Crowdstart
- Data is still visible (backer, contributions, campaign)
- User will still see their orders from Crowdstart (that were made on Crowdstart)

Public AND organization campaigns
- When a campaign has an org as a creator, all user,orders implicitly get added to that org namespace
```

# Site generation
Outline of Crowdstart/Bebop v2 architecture.

## bebop build
Takes current working dir as source dir and builds project into `dist/` folder.

Example app structure...
```sh
› tree .
.
├── css/
│   └── app.styl
├── js/
│   └── app.coffee
├── index.jade
├── referral.jade
├── products/
│   |── product.jade
│   └── index.jade
└── layout.jade
```

Transforms into this...
```
› bebop build
› tree dist
dist
├── css/
│   └── app.css
├── js/
│   └── app.js
├── referral/
│   └── index.html
├── blog/
│   |── post-1/
│   |   └── index.html
│   |── post-2/
│   |   └── index.html
├── products/
│   |── slug1/
│   |   └── index.html
│   |── slug2/
│   |   └── index.html
│   └── index.html
└── index.html
```

### Approach 1
- We replicate entire local build process on our end
- Post-processing easy, we can rebuild when DB is updated for whatever reason

### Approach 2
- Local build process gives us some IR which we can use to generate final pages
  with content from our API
- Coffee, Jade, Stylus or whatever compiles to some IR (based on html, js, css)
- Populate locally during dev in real-time

### Approach 3
- Allocate dedicated VM for build step for each company (upsell?)
- SSH access for those inclined (flip on and off managed stuff?)
- $10/mo per site?
- Always running so everything is cached and always fast

Would build local copy into some sort of IR like this:
```sh
› tree .
.
├── css/
│   └── app.css
├── js/
│   └── app.js
├── index.html
├── referral.html
├── products/
│   |── product.html
│   └── index.html
└── layout.html ?
```

Example of one file:
```jade
block content
    h1= productTitle
```

IR (for use by us with riot):
```
<!-- product.html -->

<h1>{productTitle}</h1>
```
- Rendered during development in our dev server after API call

#### Requirements
- Need to support arbitrarily complex asset pipelines
- Need to be able to merge data from our API
    - Spider blog posts and render all post pages, etc.
- Need AST for site content/site map
    - Generating sitemap.xml, etc from content, SEO
- Incremental builds
    - Use AST to reload/redeploy intelligently
- Easy to incrementally rebuild on server + invalidate/deploy when datastore changes
- Generated content not tracked by git repo (build artifact)

### Naive architecture
- Load balanced VMs with node (managed vms?)
    - 1-100 VMs
- Use Build API to trigger a rebuild
    - A: Random VM is chosen for a given build based on load-balancing strategy
    - B: Pin build VMs per client so that each rebuild can leverage locally cached content

#### Build API
#### build
- Copy cached repo/content from GCS to local vm (10GB? on disk)
    - Needs to handle arbitrarily large sites
        - Cache on central location
            - A: Use a shared persistent disk
                - Can't read + write
            - B: Mount GCS using fuse
                - Prohibitively slow?
            - C: Just copy files naively locally each time
                - Use rsync? something?
                    - Useless unless
- `bebop deps` to install whatever customer was using
    - npm install
    - bower install
    - whatever arbitrary install cruft you need
- `bebop build` to incrementally rebuild dist/ folder
    - runs asset pipeline to build minified, production ready version into dist/
- Optimize
    - git gc?
    - remove temp files?
- Cache local copy of project
- `bebop deploy` to deploy any of the updates
    - Dumps back into GCS
    - Do any cache invalidation necessary

#### deploy
- Copy cached repo/content from GCS to local vm (10GB? on disk)
- `git checkout refspec`
- `npm install` to install whatever customer was using
- `bebop build` to incrementally rebuild dist/ folder
- `bebop deploy` to deploy any of the updates
    - Dumps back into GCS
- Do any cache invalidation necessary

#### pull
- Copy cached repo/content from GCS to local vm (10GB? on disk)
- `git pull` to pull in changes or handle push or whatever
- `npm install` to install whatever customer was using
- `bebop build` to incrementally rebuild dist/ folder
- `bebop deploy` to deploy any of the updates
    - Dumps back into GCS
- Do any cache invalidation necessary

## bebop serve
```sh
$ bebop build
bebop serving cwd/ at http://localhost:1987
```
- Serve site from `dist/`, merges in data from our API automatically
- Reload on changes automatically
