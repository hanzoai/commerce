(function (global) {
  var process = {
      title: 'browser',
      browser: true,
      env: {},
      argv: [],
      nextTick: function (fn) {
        setTimeout(fn, 0)
      },
      cwd: function () {
        return '/'
      },
      chdir: function () {
      }
    };
  // Require module
  function require(file, callback) {
    if ({}.hasOwnProperty.call(require.cache, file))
      return require.cache[file];
    // Handle async require
    if (typeof callback == 'function') {
      require.load(file, callback);
      return
    }
    var resolved = require.resolve(file);
    if (!resolved)
      throw new Error('Failed to resolve module ' + file);
    var module$ = {
        id: file,
        require: require,
        filename: file,
        exports: {},
        loaded: false,
        parent: null,
        children: []
      };
    var dirname = file.slice(0, file.lastIndexOf('/') + 1);
    require.cache[file] = module$.exports;
    resolved.call(module$.exports, module$, module$.exports, dirname, file);
    module$.loaded = true;
    return require.cache[file] = module$.exports
  }
  require.modules = {};
  require.cache = {};
  require.resolve = function (file) {
    return {}.hasOwnProperty.call(require.modules, file) ? require.modules[file] : void 0
  };
  // define normal static module
  require.define = function (file, fn) {
    require.modules[file] = fn
  };
  global.require = require;
  require.define('./app', function (module, exports, __dirname, __filename) {
    var Application, page;
    page = require('page');
    Application = function () {
      function Application(state) {
        if (state == null) {
          state = {}
        }
        this.state = state;
        this._routes = {};
        this.views = []
      }
      Application.prototype.setup = function () {
        $.cookie.json = true;
        return this
      };
      Application.prototype.addRoute = function (path, cb) {
        var route;
        if ((route = this._routes[path]) == null) {
          route = new page.Route(path)
        }
        if (route.callbacks == null) {
          route.callbacks = []
        }
        route.callbacks.push(cb);
        return this._routes[path] = route
      };
      Application.prototype.setupRoutes = function () {
        var cb, k, v, _i, _len, _ref;
        _ref = this.routes;
        for (k in _ref) {
          v = _ref[k];
          if (Array.isArray(v)) {
            for (_i = 0, _len = v.length; _i < _len; _i++) {
              cb = v[_i];
              this.addRoute(k, cb)
            }
          } else {
            this.addRoute(k, v)
          }
        }
        return null
      };
      Application.prototype.dispatchRoutes = function () {
        var cb, route, _, _i, _len, _ref, _ref1;
        _ref = this._routes;
        for (_ in _ref) {
          route = _ref[_];
          if (route.regexp.test(location.pathname)) {
            _ref1 = route.callbacks;
            for (_i = 0, _len = _ref1.length; _i < _len; _i++) {
              cb = _ref1[_i];
              cb()
            }
          }
        }
        return null
      };
      Application.prototype.start = function () {
        this.setupRoutes();
        this.dispatchRoutes();
        return this
      };
      Application.prototype.get = function (k) {
        return this.state[k]
      };
      Application.prototype.set = function (k, v) {
        return this.state[k] = v
      };
      Application.prototype['delete'] = function (k) {
        return delete this.state[k]
      };
      return Application
    }();
    module.exports = function (state) {
      var app;
      app = new Application(state);
      app.setup();
      return app
    }
  });
  require.define('page', function (module, exports, __dirname, __filename) {
    ;
    (function () {
      /**
   * Perform initial dispatch.
   */
      var dispatch = true;
      /**
   * Base path.
   */
      var base = '';
      /**
   * Running flag.
   */
      var running;
      /**
   * Register `path` with callback `fn()`,
   * or route `path`, or `page.start()`.
   *
   *   page(fn);
   *   page('*', fn);
   *   page('/user/:id', load, user);
   *   page('/user/' + user.id, { some: 'thing' });
   *   page('/user/' + user.id);
   *   page();
   *
   * @param {String|Function} path
   * @param {Function} fn...
   * @api public
   */
      function page(path, fn) {
        // <callback>
        if ('function' == typeof path) {
          return page('*', path)
        }
        // route <path> to <callback ...>
        if ('function' == typeof fn) {
          var route = new Route(path);
          for (var i = 1; i < arguments.length; ++i) {
            page.callbacks.push(route.middleware(arguments[i]))
          }  // show <path> with [state]
        } else if ('string' == typeof path) {
          page.show(path, fn)  // start [options]
        } else {
          page.start(path)
        }
      }
      /**
   * Callback functions.
   */
      page.callbacks = [];
      /**
   * Get or set basepath to `path`.
   *
   * @param {String} path
   * @api public
   */
      page.base = function (path) {
        if (0 == arguments.length)
          return base;
        base = path
      };
      /**
   * Bind with the given `options`.
   *
   * Options:
   *
   *    - `click` bind to click events [true]
   *    - `popstate` bind to popstate [true]
   *    - `dispatch` perform initial dispatch [true]
   *
   * @param {Object} options
   * @api public
   */
      page.start = function (options) {
        options = options || {};
        if (running)
          return;
        running = true;
        if (false === options.dispatch)
          dispatch = false;
        if (false !== options.popstate)
          window.addEventListener('popstate', onpopstate, false);
        if (false !== options.click)
          window.addEventListener('click', onclick, false);
        if (!dispatch)
          return;
        var url = location.pathname + location.search + location.hash;
        page.replace(url, null, true, dispatch)
      };
      /**
   * Unbind click and popstate event handlers.
   *
   * @api public
   */
      page.stop = function () {
        running = false;
        removeEventListener('click', onclick, false);
        removeEventListener('popstate', onpopstate, false)
      };
      /**
   * Show `path` with optional `state` object.
   *
   * @param {String} path
   * @param {Object} state
   * @param {Boolean} dispatch
   * @return {Context}
   * @api public
   */
      page.show = function (path, state, dispatch) {
        var ctx = new Context(path, state);
        if (false !== dispatch)
          page.dispatch(ctx);
        if (!ctx.unhandled)
          ctx.pushState();
        return ctx
      };
      /**
   * Replace `path` with optional `state` object.
   *
   * @param {String} path
   * @param {Object} state
   * @return {Context}
   * @api public
   */
      page.replace = function (path, state, init, dispatch) {
        var ctx = new Context(path, state);
        ctx.init = init;
        if (null == dispatch)
          dispatch = true;
        if (dispatch)
          page.dispatch(ctx);
        ctx.save();
        return ctx
      };
      /**
   * Dispatch the given `ctx`.
   *
   * @param {Object} ctx
   * @api private
   */
      page.dispatch = function (ctx) {
        var i = 0;
        function next() {
          var fn = page.callbacks[i++];
          if (!fn)
            return unhandled(ctx);
          fn(ctx, next)
        }
        next()
      };
      /**
   * Unhandled `ctx`. When it's not the initial
   * popstate then redirect. If you wish to handle
   * 404s on your own use `page('*', callback)`.
   *
   * @param {Context} ctx
   * @api private
   */
      function unhandled(ctx) {
        var current = window.location.pathname + window.location.search;
        if (current == ctx.canonicalPath)
          return;
        page.stop();
        ctx.unhandled = true;
        window.location = ctx.canonicalPath
      }
      /**
   * Initialize a new "request" `Context`
   * with the given `path` and optional initial `state`.
   *
   * @param {String} path
   * @param {Object} state
   * @api public
   */
      function Context(path, state) {
        if ('/' == path[0] && 0 != path.indexOf(base))
          path = base + path;
        var i = path.indexOf('?');
        this.canonicalPath = path;
        this.path = path.replace(base, '') || '/';
        this.title = document.title;
        this.state = state || {};
        this.state.path = path;
        this.querystring = ~i ? path.slice(i + 1) : '';
        this.pathname = ~i ? path.slice(0, i) : path;
        this.params = [];
        // fragment
        this.hash = '';
        if (!~this.path.indexOf('#'))
          return;
        var parts = this.path.split('#');
        this.path = parts[0];
        this.hash = parts[1] || '';
        this.querystring = this.querystring.split('#')[0]
      }
      /**
   * Expose `Context`.
   */
      page.Context = Context;
      /**
   * Push state.
   *
   * @api private
   */
      Context.prototype.pushState = function () {
        history.pushState(this.state, this.title, this.canonicalPath)
      };
      /**
   * Save the context state.
   *
   * @api public
   */
      Context.prototype.save = function () {
        history.replaceState(this.state, this.title, this.canonicalPath)
      };
      /**
   * Initialize `Route` with the given HTTP `path`,
   * and an array of `callbacks` and `options`.
   *
   * Options:
   *
   *   - `sensitive`    enable case-sensitive routes
   *   - `strict`       enable strict matching for trailing slashes
   *
   * @param {String} path
   * @param {Object} options.
   * @api private
   */
      function Route(path, options) {
        options = options || {};
        this.path = path;
        this.method = 'GET';
        this.regexp = pathtoRegexp(path, this.keys = [], options.sensitive, options.strict)
      }
      /**
   * Expose `Route`.
   */
      page.Route = Route;
      /**
   * Return route middleware with
   * the given callback `fn()`.
   *
   * @param {Function} fn
   * @return {Function}
   * @api public
   */
      Route.prototype.middleware = function (fn) {
        var self = this;
        return function (ctx, next) {
          if (self.match(ctx.path, ctx.params))
            return fn(ctx, next);
          next()
        }
      };
      /**
   * Check if this route matches `path`, if so
   * populate `params`.
   *
   * @param {String} path
   * @param {Array} params
   * @return {Boolean}
   * @api private
   */
      Route.prototype.match = function (path, params) {
        var keys = this.keys, qsIndex = path.indexOf('?'), pathname = ~qsIndex ? path.slice(0, qsIndex) : path, m = this.regexp.exec(pathname);
        if (!m)
          return false;
        for (var i = 1, len = m.length; i < len; ++i) {
          var key = keys[i - 1];
          var val = 'string' == typeof m[i] ? decodeURIComponent(m[i]) : m[i];
          if (key) {
            params[key.name] = undefined !== params[key.name] ? params[key.name] : val
          } else {
            params.push(val)
          }
        }
        return true
      };
      /**
   * Normalize the given path string,
   * returning a regular expression.
   *
   * An empty array should be passed,
   * which will contain the placeholder
   * key names. For example "/user/:id" will
   * then contain ["id"].
   *
   * @param  {String|RegExp|Array} path
   * @param  {Array} keys
   * @param  {Boolean} sensitive
   * @param  {Boolean} strict
   * @return {RegExp}
   * @api private
   */
      function pathtoRegexp(path, keys, sensitive, strict) {
        if (path instanceof RegExp)
          return path;
        if (path instanceof Array)
          path = '(' + path.join('|') + ')';
        path = path.concat(strict ? '' : '/?').replace(/\/\(/g, '(?:/').replace(/(\/)?(\.)?:(\w+)(?:(\(.*?\)))?(\?)?/g, function (_, slash, format, key, capture, optional) {
          keys.push({
            name: key,
            optional: !!optional
          });
          slash = slash || '';
          return '' + (optional ? '' : slash) + '(?:' + (optional ? slash : '') + (format || '') + (capture || (format && '([^/.]+?)' || '([^/]+?)')) + ')' + (optional || '')
        }).replace(/([\/.])/g, '\\$1').replace(/\*/g, '(.*)');
        return new RegExp('^' + path + '$', sensitive ? '' : 'i')
      }
      /**
   * Handle "populate" events.
   */
      function onpopstate(e) {
        if (e.state) {
          var path = e.state.path;
          page.replace(path, e.state)
        }
      }
      /**
   * Handle "click" events.
   */
      function onclick(e) {
        if (1 != which(e))
          return;
        if (e.metaKey || e.ctrlKey || e.shiftKey)
          return;
        if (e.defaultPrevented)
          return;
        // ensure link
        var el = e.target;
        while (el && 'A' != el.nodeName)
          el = el.parentNode;
        if (!el || 'A' != el.nodeName)
          return;
        // ensure non-hash for the same path
        var link = el.getAttribute('href');
        if (el.pathname == location.pathname && (el.hash || '#' == link))
          return;
        // check target
        if (el.target)
          return;
        // x-origin
        if (!sameOrigin(el.href))
          return;
        // rebuild path
        var path = el.pathname + el.search + (el.hash || '');
        // same page
        var orig = path + el.hash;
        path = path.replace(base, '');
        if (base && orig == path)
          return;
        e.preventDefault();
        page.show(orig)
      }
      /**
   * Event button.
   */
      function which(e) {
        e = e || window.event;
        return null == e.which ? e.button : e.which
      }
      /**
   * Check if `href` is the same origin.
   */
      function sameOrigin(href) {
        var origin = location.protocol + '//' + location.hostname;
        if (location.port)
          origin += ':' + location.port;
        return 0 == href.indexOf(origin)
      }
      /**
   * Expose `page`.
   */
      if ('undefined' == typeof module) {
        window.page = page
      } else {
        module.exports = page
      }
    }())
  });
  require.define('./cart', function (module, exports, __dirname, __filename) {
    var Cart, EventEmitter, __hasProp = {}.hasOwnProperty, __extends = function (child, parent) {
        for (var key in parent) {
          if (__hasProp.call(parent, key))
            child[key] = parent[key]
        }
        function ctor() {
          this.constructor = child
        }
        ctor.prototype = parent.prototype;
        child.prototype = new ctor;
        child.__super__ = parent.prototype;
        return child
      };
    EventEmitter = require('./event-emitter');
    Cart = function (_super) {
      __extends(Cart, _super);
      function Cart(opts) {
        if (opts == null) {
          opts = {}
        }
        Cart.__super__.constructor.apply(this, arguments);
        this.cookieName = app.get('cookieName');
        this.quantity = 0;
        this.subtotal = 0;
        this.cart = {};
        this.fetch()
      }
      Cart.prototype.fetch = function () {
        var cart, _ref;
        cart = (_ref = $.cookie(this.cookieName)) != null ? _ref : {};
        if (!isNaN(cart.subtotal)) {
          this.subtotal = cart.subtotal
        }
        if (!isNaN(cart.quantity)) {
          this.quantity = cart.quantity
        }
        delete cart.quantity;
        delete cart.subtotal;
        return this.cart = cart
      };
      Cart.prototype.save = function (cart) {
        if (cart != null) {
          this.cart = cart
        }
        this.update();
        this.cart.quantity = this.quantity;
        this.cart.subtotal = this.subtotal;
        $.cookie(this.cookieName, this.cart, {
          expires: 30,
          path: '/'
        });
        delete this.cart.quantity;
        return delete this.cart.subtotal
      };
      Cart.prototype.get = function (sku) {
        return this.cart[sku]
      };
      Cart.prototype.set = function (sku, item) {
        this.cart[sku] = item;
        this.save();
        return item
      };
      Cart.prototype.items = function () {
        return this.cart
      };
      Cart.prototype.add = function (item) {
        var _item;
        if ((_item = this.get(item.sku)) == null) {
          return this.set(item.sku, item)
        }
        _item.quantity += item.quantity;
        this.quantity += item.quantity;
        this.subtotal += item.quantity * item.price;
        this.emit('quantity', this.quantity);
        this.emit('subtotal', this.subtotal);
        return _item
      };
      Cart.prototype.remove = function (sku, el) {
        delete this.cart[sku];
        return this.save()
      };
      Cart.prototype.clear = function () {
        this.cart = {};
        return this.save()
      };
      Cart.prototype.update = function () {
        var item, quantity, sku, subtotal;
        quantity = 0;
        subtotal = 0;
        for (sku in this.cart) {
          item = this.cart[sku];
          quantity += item.quantity;
          subtotal += item.price * item.quantity
        }
        this.quantity = quantity;
        this.subtotal = subtotal;
        this.emit('quantity', quantity);
        return this.emit('subtotal', subtotal)
      };
      return Cart
    }(EventEmitter);
    module.exports = new Cart
  });
  require.define('./event-emitter', function (module, exports, __dirname, __filename) {
    var EventEmitter, __slice = [].slice;
    EventEmitter = function () {
      function EventEmitter() {
        this._jQuery = $(this)
      }
      EventEmitter.prototype.emit = function () {
        var data, event;
        event = arguments[0], data = 2 <= arguments.length ? __slice.call(arguments, 1) : [];
        return this._jQuery.trigger(event, data)
      };
      EventEmitter.prototype.once = function (event, callback) {
        return this._jQuery.one(event, function (_this) {
          return function () {
            var data, event;
            event = arguments[0], data = 2 <= arguments.length ? __slice.call(arguments, 1) : [];
            return callback.apply(_this, data)
          }
        }(this))
      };
      EventEmitter.prototype.on = function (event, callback) {
        return this._jQuery.bind(event, function (_this) {
          return function () {
            var data, event;
            event = arguments[0], data = 2 <= arguments.length ? __slice.call(arguments, 1) : [];
            return callback.apply(_this, data)
          }
        }(this))
      };
      EventEmitter.prototype.off = function (event, callback) {
        return this._jQuery.unbind(event, callback)
      };
      return EventEmitter
    }();
    module.exports = EventEmitter
  });
  require.define('./views/alert', function (module, exports, __dirname, __filename) {
    var AlertView, View, __hasProp = {}.hasOwnProperty, __extends = function (child, parent) {
        for (var key in parent) {
          if (__hasProp.call(parent, key))
            child[key] = parent[key]
        }
        function ctor() {
          this.constructor = child
        }
        ctor.prototype = parent.prototype;
        child.prototype = new ctor;
        child.__super__ = parent.prototype;
        return child
      };
    View = require('./view');
    AlertView = function (_super) {
      __extends(AlertView, _super);
      AlertView.prototype.el = '.sqs-widgets-confirmation.alert';
      function AlertView(opts) {
        var _ref, _ref1, _ref2, _ref3;
        if (opts == null) {
          opts = {}
        }
        AlertView.__super__.constructor.apply(this, arguments);
        this.$nextTo = $((_ref = opts.nextTo) != null ? _ref : 'body');
        this.set('confirm', (_ref1 = opts.confirm) != null ? _ref1 : 'okay');
        this.set('message', (_ref2 = opts.message) != null ? _ref2 : 'message');
        this.set('title', (_ref3 = opts.title) != null ? _ref3 : 'title')
      }
      AlertView.prototype.bindings = {
        title: '.title',
        message: '.message',
        confirm: '.confirmation-button'
      };
      AlertView.prototype.events = {
        'mousedown document': function () {
          return this.dismiss()
        },
        'keydown document': function (e) {
          if (!e) {
            e = event
          }
          if (e.keyCode === 27) {
            return this.dismiss()
          }
        },
        'scroll window': function () {
          return this.dismiss()
        }
      };
      AlertView.prototype.show = function (opts) {
        if (opts == null) {
          opts = {}
        }
        if (opts.title != null) {
          this.set('title', opts.title)
        }
        if (opts.message != null) {
          this.set('message', opts.message)
        }
        if (opts.title != null) {
          this.set('title', opts.title)
        }
        this.render();
        this.bind();
        this.position();
        return this.$el.fadeIn(200)
      };
      AlertView.prototype.dismiss = function () {
        this.unbind();
        return this.$el.fadeOut(200, function (_this) {
          return function () {
            return _this.$el.css({ top: -1000 })
          }
        }(this))
      };
      AlertView.prototype.position = function () {
        var offset, topOffset;
        offset = this.$nextTo.offset();
        topOffset = offset.top - $(window).scrollTop();
        return this.$el.css({
          position: 'fixed',
          top: topOffset - 42 + 'px',
          left: offset.left - 66 + 'px'
        })
      };
      return AlertView
    }(View);
    module.exports = AlertView
  });
  require.define('./view', function (module, exports, __dirname, __filename) {
    var View, util, __slice = [].slice;
    util = require('./util');
    View = function () {
      View.prototype.el = null;
      View.prototype.bindings = {};
      View.prototype.computed = {};
      View.prototype.events = {};
      View.prototype.formatters = {};
      View.prototype.watching = {};
      function View(opts) {
        var name, watched, watcher, _base, _i, _j, _len, _len1, _ref, _ref1;
        if (opts == null) {
          opts = {}
        }
        if (this.el == null) {
          this.el = opts.el
        }
        if (opts.$el) {
          this.$el = opts.$el
        } else {
          if (this.template) {
            this.$el = $($(this.template).html())
          } else {
            this.$el = $(this.el)
          }
        }
        this.id = util.uniqueId(this.constructor.name);
        this.state = (_ref = opts.state) != null ? _ref : {};
        this._events = {};
        this._targets = {};
        this._watchers = {};
        _ref1 = this.watching;
        for (watched = _i = 0, _len = _ref1.length; _i < _len; watched = ++_i) {
          watcher = _ref1[watched];
          if (!Array.isArray(watched)) {
            watched = [watched]
          }
          for (_j = 0, _len1 = watched.length; _j < _len1; _j++) {
            name = watched[_j];
            if ((_base = this._watchers)[name] == null) {
              _base[name] = []
            }
            this._watchers[name].push(watcher)
          }
        }
        this._cacheTargets();
        if (!!opts.autoRender) {
          this.render()
        }
      }
      View.prototype._splitTarget = function (target) {
        var attr, selector, _ref, _ref1;
        if (target.indexOf('@' !== -1)) {
          _ref = target.split(/\s+@/), selector = _ref[0], attr = _ref[1]
        } else {
          _ref1 = [
            target,
            null
          ], selector = _ref1[0], attr = _ref1[1]
        }
        if (attr == null) {
          attr = 'text'
        }
        return [
          selector,
          attr
        ]
      };
      View.prototype._cacheTargets = function () {
        var attr, name, selector, target, targets, _ref, _results;
        _ref = this.bindings;
        _results = [];
        for (name in _ref) {
          targets = _ref[name];
          if (!Array.isArray(targets)) {
            targets = [targets]
          }
          _results.push(function () {
            var _i, _len, _ref1, _results1;
            _results1 = [];
            for (_i = 0, _len = targets.length; _i < _len; _i++) {
              target = targets[_i];
              _ref1 = this._splitTarget(target), selector = _ref1[0], attr = _ref1[1];
              if (this._targets[selector] == null) {
                _results1.push(this._targets[selector] = this.$el.find(selector))
              } else {
                _results1.push(void 0)
              }
            }
            return _results1
          }.call(this))
        }
        return _results
      };
      View.prototype._computeComputed = function (name) {
        var args, sources, src, value, _i, _j, _len, _len1, _ref;
        args = [];
        _ref = this.watching[name];
        for (_i = 0, _len = _ref.length; _i < _len; _i++) {
          sources = _ref[_i];
          if (!Array.isArray(sources)) {
            sources = [sources]
          }
          for (_j = 0, _len1 = sources.length; _j < _len1; _j++) {
            src = sources[_j];
            args.push(this.state[src])
          }
        }
        return value = this.computed[name].apply(this, args)
      };
      View.prototype._renderBindings = function (name, value) {
        var attr, formatter, selector, target, targets, _i, _len, _ref, _value;
        if (this.computed[name] != null) {
          value = this._computeComputed(name)
        }
        targets = this.bindings[name];
        if (!Array.isArray(targets)) {
          targets = [targets]
        }
        for (_i = 0, _len = targets.length; _i < _len; _i++) {
          target = targets[_i];
          _ref = this._splitTarget(target), selector = _ref[0], attr = _ref[1];
          if ((formatter = this.formatters[name]) != null) {
            _value = formatter(value, '' + selector + ' @' + attr)
          } else {
            _value = value
          }
          this._mutateDom(selector, attr, _value)
        }
      };
      View.prototype._mutateDom = function (selector, attr, value) {
        if (attr === 'text') {
          this._targets[selector].text(value)
        } else {
          this._targets[selector].attr(attr, value)
        }
      };
      View.prototype.get = function (name) {
        return this.state[name]
      };
      View.prototype.set = function (name, value) {
        var watcher, watchers, _i, _len, _results;
        this.state[name] = value;
        if (this.bindings[name] != null) {
          this._renderBindings(name, value)
        }
        if ((watchers = this._watchers[name]) != null) {
          _results = [];
          for (_i = 0, _len = watchers.length; _i < _len; _i++) {
            watcher = watchers[_i];
            _results.push(this._renderBindings(watcher))
          }
          return _results
        }
      };
      View.prototype.render = function (state) {
        var k, name, targets, v, _ref;
        if (state != null) {
          for (k in state) {
            v = state[k];
            this.set(k, v)
          }
        } else {
          _ref = this.bindings;
          for (name in _ref) {
            targets = _ref[name];
            this._renderBindings(name, this.state[name])
          }
        }
        return this
      };
      View.prototype._splitEvent = function (e) {
        var $el, event, selector, _ref;
        _ref = e.split(/\s+/), event = _ref[0], selector = 2 <= _ref.length ? __slice.call(_ref, 1) : [];
        selector = selector.join(' ');
        if (!selector) {
          $el = this.$el;
          return [
            $el,
            event
          ]
        }
        switch (selector) {
        case 'document':
          $el = $(document);
          break;
        case 'window':
          $el = $(window);
          break;
        default:
          $el = this.$el.find(selector)
        }
        return [
          $el,
          event
        ]
      };
      View.prototype.on = function (e, callback) {
        var $el, event, _ref;
        _ref = this._splitEvent(e), $el = _ref[0], event = _ref[1];
        if (typeof callback === 'string') {
          callback = this[callback]
        }
        $el.on('' + event + '.' + this.id, function (_this) {
          return function () {
            return callback.apply(_this, arguments)
          }
        }(this));
        return this
      };
      View.prototype.once = function (e, callback) {
        var $el, event, _ref;
        _ref = this._splitEvent(e), $el = _ref[0], event = _ref[1];
        if (typeof callback === 'string') {
          callback = this[callback]
        }
        $el.one('' + event + '.' + this.id, function (_this) {
          return function () {
            return callback.apply(_this, arguments)
          }
        }(this));
        return this
      };
      View.prototype.off = function (e) {
        var $el, event, _ref;
        _ref = this._splitEvent(e), $el = _ref[0], event = _ref[1];
        $el.off('' + event + '.' + this.id);
        return this
      };
      View.prototype.emit = function () {
        var $el, data, e, event, _ref;
        e = arguments[0], data = 2 <= arguments.length ? __slice.call(arguments, 1) : [];
        _ref = this._splitEvent(e), $el = _ref[0], event = _ref[1];
        $el.trigger(event, data);
        return this
      };
      View.prototype.bind = function () {
        var k, v, _ref;
        _ref = this.events;
        for (k in _ref) {
          v = _ref[k];
          this.on(k, v)
        }
        return this
      };
      View.prototype.unbind = function () {
        var k, v, _ref;
        _ref = this.events;
        for (k in _ref) {
          v = _ref[k];
          this.off(k, v)
        }
        return this
      };
      return View
    }();
    module.exports = View
  });
  require.define('./util', function (module, exports, __dirname, __filename) {
    var humanizeNumber, _idCounter;
    exports.humanizeNumber = humanizeNumber = function (num) {
      return num.toString().replace(/(\d)(?=(\d\d\d)+(?!\d))/g, '$1,')
    };
    exports.formatCurrency = function (num) {
      var currency;
      currency = num || 0;
      return humanizeNumber(currency.toFixed(2))
    };
    _idCounter = 0;
    exports.uniqueId = function (prefix) {
      var id;
      id = ++_idCounter + '';
      return prefix != null ? prefix : prefix + id
    };
    exports.numbersOnly = function (event) {
      return event.charCode >= 48 && event.charCode <= 57
    }
  });
  require.define('./routes/cart', function (module, exports, __dirname, __filename) {
    exports.click = function () {
      return $('.fixed-cart').click(function () {
        return window.location = '/cart'
      })
    };
    exports.hideHover = function () {
      return $('.fixed-cart').hide()
    };
    exports.setupHover = function () {
      var view;
      view = new (require('./views/cart-hover'));
      app.views.push(view);
      return view.listen()
    };
    exports.setupView = function () {
      var view;
      view = new (require('./views/cart'));
      app.views.push(view);
      return view.render()
    }
  });
  require.define('./views/cart-hover', function (module, exports, __dirname, __filename) {
    var CartHover, View, cart, util, __hasProp = {}.hasOwnProperty, __extends = function (child, parent) {
        for (var key in parent) {
          if (__hasProp.call(parent, key))
            child[key] = parent[key]
        }
        function ctor() {
          this.constructor = child
        }
        ctor.prototype = parent.prototype;
        child.prototype = new ctor;
        child.__super__ = parent.prototype;
        return child
      };
    View = require('./view');
    util = require('./util');
    cart = app.get('cart');
    CartHover = function (_super) {
      __extends(CartHover, _super);
      function CartHover() {
        return CartHover.__super__.constructor.apply(this, arguments)
      }
      CartHover.prototype.el = '.fixed-cart';
      CartHover.prototype.bindings = {
        quantity: '.total-quantity',
        subtotal: '.subtotal .price span',
        suffix: '.details span.suffix'
      };
      CartHover.prototype.formatters = {
        quantity: function (v) {
          return util.humanizeNumber(v)
        },
        subtotal: function (v) {
          return util.formatCurrency(v)
        },
        suffix: function (v) {
          if (v > 1) {
            return 'items'
          } else {
            return 'item'
          }
        }
      };
      CartHover.prototype.listen = function () {
        cart.on('quantity', function (_this) {
          return function (quantity) {
            _this.set('quantity', quantity);
            return _this.set('suffix', quantity)
          }
        }(this));
        cart.on('subtotal', function (_this) {
          return function (subtotal) {
            return _this.set('subtotal', subtotal)
          }
        }(this));
        this.set('quantity', cart.quantity);
        this.set('suffix', cart.quantity);
        return this.set('subtotal', cart.subtotal)
      };
      return CartHover
    }(View);
    module.exports = CartHover
  });
  require.define('./views/cart', function (module, exports, __dirname, __filename) {
    var CartView, LineItemView, View, cart, util, __hasProp = {}.hasOwnProperty, __extends = function (child, parent) {
        for (var key in parent) {
          if (__hasProp.call(parent, key))
            child[key] = parent[key]
        }
        function ctor() {
          this.constructor = child
        }
        ctor.prototype = parent.prototype;
        child.prototype = new ctor;
        child.__super__ = parent.prototype;
        return child
      };
    LineItemView = require('./views/line-item');
    View = require('./view');
    util = require('./util');
    cart = app.get('cart');
    CartView = function (_super) {
      __extends(CartView, _super);
      function CartView() {
        return CartView.__super__.constructor.apply(this, arguments)
      }
      CartView.prototype.el = '.sqs-fullpage-shopping-cart-content';
      CartView.prototype.bindings = { subtotal: '.subtotal .price span' };
      CartView.prototype.formatters = {
        subtotal: function (v) {
          return util.formatCurrency(v)
        }
      };
      CartView.prototype.render = function () {
        var index, item, sku, view, _ref;
        $('.cart-container tbody').html('');
        index = 0;
        this.set('quantity', cart.quantity);
        this.set('subtotal', cart.subtotal);
        _ref = cart.items();
        for (sku in _ref) {
          item = _ref[sku];
          item.index = ++index;
          view = new LineItemView({ state: item });
          window.view = view;
          view.render();
          view.bind();
          $('.cart-container tbody').append(view.$el)
        }
        return cart.on('subtotal', function (_this) {
          return function (subtotal) {
            return _this.set('subtotal', subtotal)
          }
        }(this))
      };
      return CartView
    }(View);
    module.exports = CartView
  });
  require.define('./views/line-item', function (module, exports, __dirname, __filename) {
    var LineItemView, View, util, __hasProp = {}.hasOwnProperty, __extends = function (child, parent) {
        for (var key in parent) {
          if (__hasProp.call(parent, key))
            child[key] = parent[key]
        }
        function ctor() {
          this.constructor = child
        }
        ctor.prototype = parent.prototype;
        child.prototype = new ctor;
        child.__super__ = parent.prototype;
        return child
      };
    View = require('./view');
    util = require('./util');
    LineItemView = function (_super) {
      __extends(LineItemView, _super);
      function LineItemView() {
        return LineItemView.__super__.constructor.apply(this, arguments)
      }
      LineItemView.prototype.template = '#line-item-template';
      LineItemView.prototype.bindings = {
        img: 'img.thumbnail   @src',
        sku: 'input.sku       @value',
        slug: 'input.slug      @value',
        name: 'a.title',
        desc: 'div.desc',
        price: '.price span',
        quantity: '.quantity input @value',
        index: [
          'input.sku       @name',
          'input.slug      @name',
          '.quantity input @name'
        ]
      };
      LineItemView.prototype.computed = {
        desc: function (color, size) {
          return [
            color,
            size
          ]
        }
      };
      LineItemView.prototype.watching = {
        desc: [
          'color',
          'size'
        ]
      };
      LineItemView.prototype.formatters = {
        desc: function (v) {
          if (v.length > 1) {
            return v.join(' / ')
          } else {
            return v.join('')
          }
        },
        index: function (v, selector) {
          switch (selector) {
          case 'input.sku @name':
            return 'Order.Items.' + v + '.Variant.SKU';
          case 'input.slug @name':
            return 'Order.Items.' + v + '.Product.Slug';
          case '.quantity input @name':
            return 'Order.Items.' + v + '.Quantity'
          }
        },
        price: function (v) {
          return util.formatCurrency(v)
        }
      };
      LineItemView.prototype.events = {
        'change .quantity input': 'updateQuantity',
        'keypress input,select': function (e) {
          if (e.keyCode !== 13) {
            return true
          } else {
            this.updateQuantity(e);
            return false
          }
        },
        'click .remove-item': function () {
          var cart;
          cart = app.get('cart');
          cart.remove(this.state.sku);
          return this.destroy()
        }
      };
      LineItemView.prototype.updateQuantity = function (e) {
        var cart, el, quantity;
        el = $(e.currentTarget);
        e.preventDefault();
        e.stopPropagation();
        quantity = parseInt(el.val(), 10);
        if (quantity < 1 || isNaN(quantity)) {
          quantity = 1
        }
        this.set('quantity', quantity);
        cart = app.get('cart');
        return cart.set(this.state.sku, this.state)
      };
      LineItemView.prototype.destroy = function () {
        this.unbind();
        return this.$el.animate({ opacity: 'toggle' }, 500, 'swing', function (_this) {
          return function () {
            return _this.$el.remove()
          }
        }(this))
      };
      return LineItemView
    }(View);
    module.exports = LineItemView
  });
  require.define('./routes/products', function (module, exports, __dirname, __filename) {
    var ProductView;
    ProductView = require('./views/product');
    exports.setupView = function () {
      var view;
      view = new ProductView;
      app.views.push(view);
      return view.bind()
    };
    exports.gallery = function () {
      return $('#productThumbnails .slide img').each(function (i, v) {
        return $(v).click(function () {
          var src;
          src = $(v).data('src');
          return $('#productSlideshow .slide img').each(function (i, v) {
            if (src === $(v).data('src')) {
              return $(v).fadeIn(400)
            } else {
              return $(v).fadeOut(400)
            }
          })
        })
      })
    };
    exports.customizeAr1 = function () {
      var $slides, i;
      $slides = function () {
        var _i, _len, _ref, _results;
        _ref = $('#productSlideshow .slide img');
        _results = [];
        for (_i = 0, _len = _ref.length; _i < _len; _i++) {
          i = _ref[_i];
          _results.push($(i))
        }
        return _results
      }();
      return $('[data-variant-option-name=Color]').change(function () {
        if ($(this).val() === 'Black') {
          $slides[0].fadeIn();
          return $slides[1].fadeOut()
        } else {
          $slides[1].fadeIn();
          return $slides[0].fadeOut()
        }
      })
    }
  });
  require.define('./views/product', function (module, exports, __dirname, __filename) {
    var ProductView, View, cart, __hasProp = {}.hasOwnProperty, __extends = function (child, parent) {
        for (var key in parent) {
          if (__hasProp.call(parent, key))
            child[key] = parent[key]
        }
        function ctor() {
          this.constructor = child
        }
        ctor.prototype = parent.prototype;
        child.prototype = new ctor;
        child.__super__ = parent.prototype;
        return child
      };
    View = require('./view');
    cart = app.get('cart');
    ProductView = function (_super) {
      __extends(ProductView, _super);
      function ProductView() {
        return ProductView.__super__.constructor.apply(this, arguments)
      }
      ProductView.prototype.el = '.sqs-add-to-cart-button';
      ProductView.prototype.events = {
        click: function () {
          return this.addToCart()
        }
      };
      ProductView.prototype.addToCart = function () {
        var inner, quantity, variant;
        if ((variant = this.getVariant()) == null) {
          return
        }
        quantity = parseInt($('#quantity').val(), 10);
        inner = $('.sqs-add-to-cart-button-inner');
        inner.html('');
        inner.append('<div class="yui3-widget sqs-spin light"></div>');
        inner.append('<div class="status-text">Adding...</div>');
        cart.add({
          sku: variant.SKU,
          color: variant.Color,
          img: currentProduct.Images[0].Url,
          name: currentProduct.Title,
          price: parseInt(variant.Price, 10) * 0.0001,
          quantity: quantity,
          size: variant.Size,
          slug: currentProduct.Slug
        });
        setTimeout(function () {
          return $('.status-text').text('Added!').fadeOut(500, function () {
            return inner.html('Add to Cart')
          })
        }, 500);
        return setTimeout(function () {
          return $('.sqs-pill-shopping-cart-content').animate({ opacity: 0.85 }, 400, function () {
            return $('.sqs-pill-shopping-cart-content').animate({ opacity: 1 }, 300)
          })
        }, 300)
      };
      ProductView.prototype.getVariant = function () {
        var alert, missingOptions, optionsMatch, selected, variant, variants, _i, _len;
        selected = {};
        variants = currentProduct.Variants;
        missingOptions = [];
        optionsMatch = function (selected, variant) {
          var k, v;
          for (k in selected) {
            v = selected[k];
            if (variant[k] !== selected[k]) {
              return false
            }
          }
          return true
        };
        $('.variant-option').each(function (i, v) {
          $(v).find('select').each(function (i, v) {
            var $select, name, value;
            $select = $(v);
            name = $select.data('variant-option-name');
            value = $select.val();
            selected[name] = value;
            if (value === 'none') {
              missingOptions.push(name)
            }
          })
        });
        if (missingOptions.length > 0) {
          alert = app.get('alert');
          alert.show({
            title: 'Unable To Add Item',
            message: 'Please select a ' + missingOptions[0] + ' option.',
            confirm: 'Okay'
          });
          return
        }
        for (_i = 0, _len = variants.length; _i < _len; _i++) {
          variant = variants[_i];
          if (optionsMatch(selected, variant)) {
            return variant
          }
        }
        return variants[0]
      };
      return ProductView
    }(View);
    module.exports = ProductView
  });
  require.define('./crowdstart', function (module, exports, __dirname, __filename) {
    var app, cart, products;
    window.app = app = require('./app')({ cookieName: 'SKULLYSystemsCart' });
    app.set('cart', require('./cart'));
    app.set('alert', new (require('./views/alert'))({ nextTo: '.sqs-add-to-cart-button' }));
    cart = require('./routes/cart');
    products = require('./routes/products');
    app.routes = {
      '/cart': [
        cart.hideHover,
        cart.setupView
      ],
      '/products/*': [
        products.setupView,
        products.gallery,
        cart.setupHover
      ],
      '/products/ar-1': [products.customizeAr1],
      '/': [cart.setupHover],
      '*': cart.click
    };
    app.start()
  });
  require('./crowdstart')
}.call(this, this))