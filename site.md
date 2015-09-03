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
