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
  require.define('mvstar/lib/app', function (module, exports, __dirname, __filename) {
    var App, Route;
    Route = require('mvstar/lib/route');
    App = function () {
      function App(state) {
        if (state == null) {
          state = {}
        }
        this.state = state;
        this._routes = {};
        this.views = []
      }
      App.prototype.addRoute = function (path, cb) {
        var route;
        if ((route = this._routes[path]) == null) {
          route = new Route(path)
        }
        if (route.callbacks == null) {
          route.callbacks = []
        }
        route.callbacks.push(cb);
        return this._routes[path] = route
      };
      App.prototype.setupRoutes = function () {
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
      App.prototype.dispatchRoutes = function () {
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
      App.prototype.start = function () {
        this.setupRoutes();
        this.dispatchRoutes();
        return this
      };
      App.prototype.get = function (k) {
        return this.state[k]
      };
      App.prototype.set = function (k, v) {
        return this.state[k] = v
      };
      App.prototype['delete'] = function (k) {
        return delete this.state[k]
      };
      return App
    }();
    module.exports = App  //# sourceMappingURL=app.js.map
  });
  require.define('mvstar/lib/route', function (module, exports, __dirname, __filename) {
    var Route, pathtoRegexp;
    pathtoRegexp = require('mvstar/node_modules/path-to-regexp');
    Route = function () {
      function Route(path, options) {
        if (options == null) {
          options = {}
        }
        if (path === '*') {
          this.path = '(.*)'
        } else {
          this.path = path
        }
        this.keys = [];
        this.regexp = pathtoRegexp(this.path, this.keys, options.sensitive, options.strict)
      }
      return Route
    }();
    module.exports = Route  //# sourceMappingURL=route.js.map
  });
  require.define('mvstar/node_modules/path-to-regexp', function (module, exports, __dirname, __filename) {
    module.exports = pathtoRegexp;
    /**
 * The main path matching regexp utility.
 *
 * @type {RegExp}
 */
    var PATH_REGEXP = new RegExp([
      // Match already escaped characters that would otherwise incorrectly appear
      // in future matches. This allows the user to escape special characters that
      // shouldn't be transformed.
      '(\\\\.)',
      // Match Express-style parameters and un-named parameters with a prefix
      // and optional suffixes. Matches appear as:
      //
      // "/:test(\\d+)?" => ["/", "test", "\d+", undefined, "?"]
      // "/route(\\d+)" => [undefined, undefined, undefined, "\d+", undefined]
      '([\\/.])?(?:\\:(\\w+)(?:\\(((?:\\\\.|[^)])*)\\))?|\\(((?:\\\\.|[^)])*)\\))([+*?])?',
      // Match regexp special characters that should always be escaped.
      '([.+*?=^!:${}()[\\]|\\/])'
    ].join('|'), 'g');
    /**
 * Escape the capturing group by escaping special characters and meaning.
 *
 * @param  {String} group
 * @return {String}
 */
    function escapeGroup(group) {
      return group.replace(/([=!:$\/()])/g, '\\$1')
    }
    /**
 * Attach the keys as a property of the regexp.
 *
 * @param  {RegExp} re
 * @param  {Array}  keys
 * @return {RegExp}
 */
    var attachKeys = function (re, keys) {
      re.keys = keys;
      return re
    };
    /**
 * Normalize the given path string, returning a regular expression.
 *
 * An empty array should be passed in, which will contain the placeholder key
 * names. For example `/user/:id` will then contain `["id"]`.
 *
 * @param  {(String|RegExp|Array)} path
 * @param  {Array}                 keys
 * @param  {Object}                options
 * @return {RegExp}
 */
    function pathtoRegexp(path, keys, options) {
      if (keys && !Array.isArray(keys)) {
        options = keys;
        keys = null
      }
      keys = keys || [];
      options = options || {};
      var strict = options.strict;
      var end = options.end !== false;
      var flags = options.sensitive ? '' : 'i';
      var index = 0;
      if (path instanceof RegExp) {
        // Match all capturing groups of a regexp.
        var groups = path.source.match(/\((?!\?)/g) || [];
        // Map all the matches to their numeric keys and push into the keys.
        keys.push.apply(keys, groups.map(function (match, index) {
          return {
            name: index,
            delimiter: null,
            optional: false,
            repeat: false
          }
        }));
        // Return the source back to the user.
        return attachKeys(path, keys)
      }
      if (Array.isArray(path)) {
        // Map array parts into regexps and return their source. We also pass
        // the same keys and options instance into every generation to get
        // consistent matching groups before we join the sources together.
        path = path.map(function (value) {
          return pathtoRegexp(value, keys, options).source
        });
        // Generate a new regexp instance by joining all the parts together.
        return attachKeys(new RegExp('(?:' + path.join('|') + ')', flags), keys)
      }
      // Alter the path string into a usable regexp.
      path = path.replace(PATH_REGEXP, function (match, escaped, prefix, key, capture, group, suffix, escape) {
        // Avoiding re-escaping escaped characters.
        if (escaped) {
          return escaped
        }
        // Escape regexp special characters.
        if (escape) {
          return '\\' + escape
        }
        var repeat = suffix === '+' || suffix === '*';
        var optional = suffix === '?' || suffix === '*';
        keys.push({
          name: key || index++,
          delimiter: prefix || '/',
          optional: optional,
          repeat: repeat
        });
        // Escape the prefix character.
        prefix = prefix ? '\\' + prefix : '';
        // Match using the custom capturing group, or fallback to capturing
        // everything up to the next slash (or next period if the param was
        // prefixed with a period).
        capture = escapeGroup(capture || group || '[^' + (prefix || '\\/') + ']+?');
        // Allow parameters to be repeated more than once.
        if (repeat) {
          capture = capture + '(?:' + prefix + capture + ')*'
        }
        // Allow a parameter to be optional.
        if (optional) {
          return '(?:' + prefix + '(' + capture + '))?'
        }
        // Basic parameter support.
        return prefix + '(' + capture + ')'
      });
      // Check whether the path ends in a slash as it alters some match behaviour.
      var endsWithSlash = path[path.length - 1] === '/';
      // In non-strict mode we allow an optional trailing slash in the match. If
      // the path to match already ended with a slash, we need to remove it for
      // consistency. The slash is only valid at the very end of a path match, not
      // anywhere in the middle. This is important for non-ending mode, otherwise
      // "/test/" will match "/test//route".
      if (!strict) {
        path = (endsWithSlash ? path.slice(0, -2) : path) + '(?:\\/(?=$))?'
      }
      // In non-ending mode, we need prompt the capturing groups to match as much
      // as possible by using a positive lookahead for the end or next path segment.
      if (!end) {
        path += strict && endsWithSlash ? '' : '(?=\\/|$)'
      }
      return attachKeys(new RegExp('^' + path + (end ? '$' : ''), flags), keys)
    }
    ;
  });
  require.define('./variants', function (module, exports, __dirname, __filename) {
    module.exports = { skus: [] }
  });
  require.define('./preorder', function (module, exports, __dirname, __filename) {
    var App, PreorderApp, app, __hasProp = {}.hasOwnProperty, __extends = function (child, parent) {
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
    App = require('mvstar/lib/app');
    PreorderApp = function (_super) {
      __extends(PreorderApp, _super);
      function PreorderApp() {
        return PreorderApp.__super__.constructor.apply(this, arguments)
      }
      PreorderApp.prototype.start = function () {
        PreorderApp.__super__.start.apply(this, arguments);
        return $.cookie.json = true
      };
      return PreorderApp
    }(App);
    window.app = app = new PreorderApp;
    app.set('variants', require('./variants'));
    app.routes = { '/order/*': [] };
    app.start()
  });
  require('./preorder')
}.call(this, this))