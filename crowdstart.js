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
    var Application, page, __slice = [].slice;
    page = require('page');
    Application = function () {
      function Application(state) {
        if (state == null) {
          state = {}
        }
        this.state = state
      }
      Application.prototype.setup = function () {
        return $.cookie.json = true
      };
      Application.prototype.setupRouting = function () {
        var k, v, _ref, _results;
        _ref = this.routes;
        _results = [];
        for (k in _ref) {
          v = _ref[k];
          if (Array.isArray(v)) {
            _results.push(page.apply.apply(page, [k].concat(__slice.call(v))))
          } else {
            _results.push(page(k, v))
          }
        }
        return _results
      };
      Application.prototype.start = function () {
        this.setupRoutes();
        return page.start()
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
  require.define('./routes/cart', function (module, exports, __dirname, __filename) {
    var cart;
    cart = require('./cart');
    exports.click = function () {
      return $('.fixed-cart').click(function () {
        return window.location = '/cart'
      })
    };
    exports.hideHover = function () {
      return $('.fixed-cart').hide()
    };
    exports.updateHover = function () {
      return cart.updateHover()
    }
  });
  require.define('./cart', function (module, exports, __dirname, __filename) {
    var product, templateEl;
    product = require('./product');
    exports.setCart = function (cart) {
      $.cookie(csio.cookieName, cart, {
        expires: 30,
        path: '/'
      })
    };
    exports.getCart = function () {
      return $.cookie(csio.cookieName) || {}
    };
    exports.clearCart = function () {
      $.cookie(csio.cookieName, {}, {
        expires: 30,
        path: '/'
      })
    };
    exports.updateHover = function (modifiedCart) {
      var cart, k, lineItem, numItems, subTotal;
      cart = modifiedCart || csio.getCart();
      numItems = 0;
      subTotal = 0;
      for (k in cart) {
        lineItem = cart[k];
        numItems += lineItem.quantity;
        subTotal += lineItem.price * lineItem.quantity
      }
      $('.total-quantity').text(util.humanizeNumber(numItems));
      $('.subtotal .price span').text(util.formatCurrency(subTotal));
      if (numItems === 1) {
        $('.details span.suffix').text('item')
      } else {
        $('.details span.suffix').text('items')
      }
    };
    templateEl = $('.template');
    templateEl.parent().remove();
    csio.renderLineItem = function (lineItem, index) {
      var $quantity, el, variantInfo;
      el = templateEl.clone(false);
      $quantity = el.find('.quantity input');
      variantInfo = [];
      if (lineItem.color !== '') {
        variantInfo.push(lineItem.color)
      }
      if (lineItem.size !== '') {
        variantInfo.push(lineItem.size)
      }
      el.find('img.thumbnail').attr('src', lineItem.img);
      el.find('input.slug').val(lineItem.slug).attr('name', 'Order.Items.' + index + '.Product.Slug');
      el.find('input.sku').val(lineItem.sku).attr('name', 'Order.Items.' + index + '.Variant.SKU');
      el.find('a.title').text(lineItem.name);
      el.find('div.variant-info').text(variantInfo.join(' / '));
      el.find('.price span').text(formatCurrency(lineItem.price));
      $quantity.val(lineItem.quantity).attr('name', 'Order.Items.' + index + '.Quantity');
      $quantity.change(function (e) {
        var quantity;
        e.preventDefault();
        e.stopPropagation();
        quantity = parseInt($(this).val(), 10);
        if (quantity < 1) {
          quantity = 1;
          $(this).val(1)
        }
        lineItem.quantity = quantity;
        csio.updateLineItem(lineItem, el)
      });
      el.find('.remove-item').click(function () {
        csio.removeLineItem(lineItem.sku, el)
      });
      el.removeClass('template');
      $('.cart-container tbody').append(el)
    };
    exports.renderCart = function (modifiedCart) {
      var cart, i, k, lineItem, numItems, subtotal;
      cart = modifiedCart || csio.getCart();
      numItems = 0;
      subtotal = 0;
      i = 0;
      $('.cart-container tbody').html('');
      for (k in cart) {
        lineItem = cart[k];
        numItems += lineItem.quantity;
        subtotal += lineItem.price * lineItem.quantity;
        csio.renderLineItem(lineItem, i);
        i += 1
      }
      if (i === 0) {
        $('.cart-container').hide();
        $('.empty-message').show()
      } else {
        csio.updateSubtotal(subtotal)
      }
    };
    csio.getSubtotal = function () {
      var cart, k, subtotal;
      subtotal = 0;
      cart = csio.getCart();
      for (k in cart) {
        subtotal += cart[k].quantity * cart[k].price
      }
      return subtotal
    };
    csio.updateSubtotal = function (subtotal) {
      subtotal = subtotal || csio.getSubtotal();
      $('.subtotal .price span').text(formatCurrency(subtotal))
    };
    csio.removeLineItem = function (sku, el) {
      var cart;
      cart = csio.getCart();
      delete cart[sku];
      csio.setCart(cart);
      csio.updateSubtotal();
      $(el).fadeOut(function () {
        $(el).remove()
      })
    };
    csio.updateLineItem = function (lineItem) {
      var cart;
      cart = csio.getCart();
      cart[lineItem.sku] = lineItem;
      csio.setCart(cart);
      csio.updateSubtotal()
    };
    $('input,select').keypress(function (e) {
      return e.keyCode !== 13
    });
    csio.addToCart = function () {
      var cart, inner, quantity, sku, variant;
      quantity = parseInt($('#quantity').val(), 10);
      cart = $.cookie(csio.cookieName) || {};
      variant = product.getVariant();
      if (variant == null) {
        return
      }
      sku = variant.SKU;
      if (cart[sku]) {
        cart[sku].quantity += quantity
      } else {
        cart[sku] = {
          sku: variant.SKU,
          color: variant.Color,
          img: csio.currentProduct.Images[0].Url,
          name: csio.currentProduct.Title,
          quantity: quantity,
          size: variant.Size,
          price: parseInt(variant.Price, 10) * 0.0001,
          slug: csio.currentProduct.Slug
        }
      }
      csio.setCart(cart);
      inner = $('.sqs-add-to-cart-button-inner');
      inner.html('');
      inner.append('<div class="yui3-widget sqs-spin light" ></div>');
      inner.append('<div class="status-text">Adding...</div>');
      setTimeout(function () {
        return $('.status-text').text('Added!').fadeOut(500, function () {
          return inner.html('Add to Cart')
        })
      }, 500);
      return setTimeout(function () {
        return $('.sqs-pill-shopping-cart-content').animate({ opacity: 0.85 }, 400, function () {
          csio.updateCartHover(cart);
          return $('.sqs-pill-shopping-cart-content').animate({ opacity: 1 }, 300)
        })
      }, 300)
    }
  });
  require.define('./product', function (module, exports, __dirname, __filename) {
    var alert;
    alert = require('./alert');
    exports.getVariant = function () {
      var missingOptions, optionsMatch, selected, variant, variants, _i, _len;
      selected = {};
      variants = csio.currentProduct.Variants;
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
        return alert({
          title: 'Unable To Add Item',
          message: 'Please select a ' + missingOptions[0] + ' option.',
          confirm: 'Okay',
          nextTo: '.sqs-add-to-cart-button'
        }).show()
      }
      for (_i = 0, _len = variants.length; _i < _len; _i++) {
        variant = variants[_i];
        if (optionsMatch(selected, variant)) {
          return variant
        }
      }
      return variants[0]
    }
  });
  require.define('./alert', function (module, exports, __dirname, __filename) {
    var Alert, View, __hasProp = {}.hasOwnProperty, __extends = function (child, parent) {
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
    Alert = function (_super) {
      __extends(Alert, _super);
      Alert.prototype.el = '.sqs-widgets-confirmation.alert';
      function Alert(opts) {
        var _ref, _ref1, _ref2, _ref3;
        if (opts == null) {
          opts = {}
        }
        Alert.__super__.constructor.apply(this, arguments);
        this.$nextTo = $((_ref = opts.nextTo) != null ? _ref : 'body');
        this.state.confirm = (_ref1 = opts.confirm) != null ? _ref1 : 'okay';
        this.state.message = (_ref2 = opts.message) != null ? _ref2 : 'message';
        this.state.title = (_ref3 = opts.title) != null ? _ref3 : 'title'
      }
      Alert.prototype.bindings = {
        '.title': 'title',
        '.message': 'message',
        '.confirmation-button': 'confirm'
      };
      Alert.prototype.events = {
        'mousedown document': function () {
          return this.dismiss()
        },
        'keydown document': function () {
          var e;
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
      Alert.prototype.show = function () {
        this.render();
        this.position();
        return this.$el.fadeIn(200)
      };
      Alert.prototype.dismiss = function () {
        return this.$el.fadeOut(200, function (_this) {
          return function () {
            return _this.$el.css({ top: -1000 })
          }
        }(this))
      };
      Alert.prototype.position = function () {
        var offset, topOffset;
        offset = this.$nextTo.offset();
        topOffset = offset.top - $(window).scrollTop();
        return this.$el.css({
          position: 'fixed',
          top: topOffset - 42 + 'px',
          left: offset.left - 66 + 'px'
        })
      };
      return Alert
    }(View);
    module.exports = function (opts) {
      return new Alert(opts)
    }
  });
  require.define('./view', function (module, exports, __dirname, __filename) {
    var View, util, __slice = [].slice;
    util = require('./util');
    View = function () {
      function View(opts) {
        var k, v, _ref, _ref1;
        if (opts == null) {
          opts = {}
        }
        if (this.el == null) {
          this.el = opts.el
        }
        if (this.$el == null) {
          this.$el = $(this.el)
        }
        this.id = util.uniqueId(this.constructor.name);
        this.state = (_ref = opts.state) != null ? _ref : {};
        this._events = {};
        this.bindingsReverse = {};
        _ref1 = this.bindings;
        for (k in _ref1) {
          v = _ref1[k];
          this.bindingsReverse[v] = k
        }
        this._cacheDatabindEls();
        if (!!opts.autoRender) {
          this.render()
        }
      }
      View.prototype.get = function (k) {
        return this.state[k]
      };
      View.prototype.set = function (k, v) {
        this.state[k] = v;
        return this._databindEls[this.bindingsReverse[k]].text(v)
      };
      View.prototype._cacheDatabindEls = function () {
        var k, v, _ref, _results;
        if (this._databindEls != null) {
          return
        }
        this._databindEls = {};
        _ref = this.bindings;
        _results = [];
        for (k in _ref) {
          v = _ref[k];
          _results.push(this._databindEls[k] = this.$el.find(k))
        }
        return _results
      };
      View.prototype.render = function (state) {
        var k, v, _ref, _results;
        for (k in state) {
          v = state[k];
          this.state[k] = v
        }
        _ref = this.bindings;
        _results = [];
        for (k in _ref) {
          v = _ref[k];
          _results.push(this._databindEls[k].text(this.state[v]))
        }
        return _results
      };
      View.prototype._splitEvent = function (event) {
        var $el, selector, _ref;
        _ref = event.split(/\s+/), event = _ref[0], selector = _ref[1];
        if (!selector) {
          $el = this.$el;
          return [
            $el,
            event
          ]
        }
        if (/^document$|^window$/.test(selector)) {
          $el = $(selector)
        } else {
          $el = this.$el.find(selector)
        }
        return [
          $el,
          event
        ]
      };
      View.prototype.on = function (event, callback) {
        var $el, eventName, _ref;
        this._events[event] = callback;
        _ref = this._splitEvent(event), $el = _ref[0], eventName = _ref[1];
        return $el.on('' + event + '.' + this.id, function (_this) {
          return function () {
            return callback.apply(_this, arguments)
          }
        }(this))
      };
      View.prototype.off = function (event) {
        var $el, callback, k, v, _ref, _ref1, _ref2, _results;
        if (event) {
          callback = this._events[event];
          _ref = this._splitEvent(event), $el = _ref[0], event = _ref[1];
          return $el.off('' + event + '.' + this.id, callback)
        } else {
          _ref1 = this._events;
          _results = [];
          for (k in _ref1) {
            v = _ref1[k];
            _ref2 = this._splitEvent(k), $el = _ref2[0], event = _ref2[1];
            _results.push($el.off('' + event + '.' + this.id, v))
          }
          return _results
        }
      };
      View.prototype.trigger = function () {
        var $el, event, params, _ref;
        event = arguments[0], params = 2 <= arguments.length ? __slice.call(arguments, 1) : [];
        _ref = this._splitEvent(event), $el = _ref[0], event = _ref[1];
        return $el.trigger.apply($el, [event].concat(__slice.call(params)))
      };
      View.prototype.bind = function () {
        var k, v, _ref;
        _ref = this.events;
        for (k in _ref) {
          v = _ref[k];
          this.on(k, v)
        }
      };
      View.prototype.unbind = function () {
        var k, v, _ref;
        _ref = this.events;
        for (k in _ref) {
          v = _ref[k];
          this.off(k, v)
        }
      };
      return View
    }();
    module.exports = View
  });
  require.define('./util', function (module, exports, __dirname, __filename) {
    var _idCounter;
    exports.humanizeNumber = function (num) {
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
  require.define('./routes/products', function (module, exports, __dirname, __filename) {
    exports.gallery = function () {
      var thumb, _i, _len, _ref, _results;
      _ref = $('#productThumbnails .slide img');
      _results = [];
      for (_i = 0, _len = _ref.length; _i < _len; _i++) {
        thumb = _ref[_i];
        _results.push(thumb.click(function () {
          var img, src, _j, _len1, _ref1, _results1;
          src = img.data('src');
          _ref1 = $('#productSlideshow .slide img');
          _results1 = [];
          for (_j = 0, _len1 = _ref1.length; _j < _len1; _j++) {
            img = _ref1[_j];
            if (src === img.data('src')) {
              _results1.push(img.fadeIn(400))
            } else {
              _results1.push(img.fadeOut(400))
            }
          }
          return _results1
        }))
      }
      return _results
    };
    exports.customizeAr1 = function () {
      var $slides;
      $slides = $('#productSlideshow .slide img');
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
  require.define('./crowdstart', function (module, exports, __dirname, __filename) {
    var app, cart, products;
    app = require('./app')({ cookieName: 'SKULLYSystemsCart' });
    cart = require('./routes/cart');
    products = require('./routes/products');
    app.routes = {
      '/cart': cart.hideHover,
      '/products/*': products.gallery,
      '/products/ar-1': products.customizeAr1,
      '*': [
        cart.click,
        cart.updateHover
      ]
    };
    app.start()
  });
  require('./crowdstart')
}.call(this, this))