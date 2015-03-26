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
  require.define('crowdstart.js/src', function (module, exports, __dirname, __filename) {
    'use strict';
    var Crowdstart, global, xhr;
    xhr = require('crowdstart/node_modules/xhr/index.js');
    Crowdstart = function () {
      Crowdstart.prototype.endpoint = 'https://api.crowdstart.com';
      function Crowdstart(key) {
        this.key = key
      }
      Crowdstart.prototype.setKey = function (key) {
        return this.key = key
      };
      Crowdstart.prototype.req = function (uri, data, cb) {
        return xhr({
          uri: this.endpoint + uri,
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': this.key
          },
          body: JSON.stringify(data)
        }, function (err, res, body) {
          return cb(status, JSON.parse(body))
        })
      };
      Crowdstart.prototype.authorize = function (data, cb) {
        return this.req('/authorize', data, cb)
      };
      Crowdstart.prototype.charge = function (data, cb) {
        return this.req('/charge', data, cb)
      };
      return Crowdstart
    }();
    if (typeof window !== 'undefined') {
      global = window
    }
    module.exports = global.Crowdstart = new Crowdstart
  });
  require.define('crowdstart/node_modules/xhr/index.js', function (module, exports, __dirname, __filename) {
    'use strict';
    'use strict';
    var window = require('crowdstart/node_modules/xhr/node_modules/global/window.js');
    var once = require('crowdstart/node_modules/xhr/node_modules/once/once.js');
    var parseHeaders = require('crowdstart/node_modules/xhr/node_modules/parse-headers/parse-headers.js');
    var XHR = window.XMLHttpRequest || noop;
    var XDR = 'withCredentials' in new XHR ? XHR : window.XDomainRequest;
    module.exports = createXHR;
    function createXHR(options, callback) {
      function readystatechange() {
        if (xhr.readyState === 4) {
          loadFunc()
        }
      }
      function getBody() {
        // Chrome with requestType=blob throws errors arround when even testing access to responseText
        var body = undefined;
        if (xhr.response) {
          body = xhr.response
        } else if (xhr.responseType === 'text' || !xhr.responseType) {
          body = xhr.responseText || xhr.responseXML
        }
        if (isJson) {
          try {
            body = JSON.parse(body)
          } catch (e) {
          }
        }
        return body
      }
      var failureResponse = {
        body: undefined,
        headers: {},
        statusCode: 0,
        method: method,
        url: uri,
        rawRequest: xhr
      };
      function errorFunc(evt) {
        clearTimeout(timeoutTimer);
        if (!(evt instanceof Error)) {
          evt = new Error('' + (evt || 'unknown'))
        }
        evt.statusCode = 0;
        callback(evt, failureResponse)
      }
      // will load the data & process the response in a special response object
      function loadFunc() {
        clearTimeout(timeoutTimer);
        var status = xhr.status === 1223 ? 204 : xhr.status;
        var response = failureResponse;
        var err = null;
        if (status !== 0) {
          response = {
            body: getBody(),
            statusCode: status,
            method: method,
            headers: {},
            url: uri,
            rawRequest: xhr
          };
          if (xhr.getAllResponseHeaders) {
            //remember xhr can in fact be XDR for CORS in IE
            response.headers = parseHeaders(xhr.getAllResponseHeaders())
          }
        } else {
          err = new Error('Internal XMLHttpRequest Error')
        }
        callback(err, response, response.body)
      }
      if (typeof options === 'string') {
        options = { uri: options }
      }
      options = options || {};
      if (typeof callback === 'undefined') {
        throw new Error('callback argument missing')
      }
      callback = once(callback);
      var xhr = options.xhr || null;
      if (!xhr) {
        if (options.cors || options.useXDR) {
          xhr = new XDR
        } else {
          xhr = new XHR
        }
      }
      var key;
      var uri = xhr.url = options.uri || options.url;
      var method = xhr.method = options.method || 'GET';
      var body = options.body || options.data;
      var headers = xhr.headers = options.headers || {};
      var sync = !!options.sync;
      var isJson = false;
      var timeoutTimer;
      if ('json' in options) {
        isJson = true;
        headers['Accept'] || (headers['Accept'] = 'application/json');
        //Don't override existing accept header declared by user
        if (method !== 'GET' && method !== 'HEAD') {
          headers['Content-Type'] = 'application/json';
          body = JSON.stringify(options.json)
        }
      }
      xhr.onreadystatechange = readystatechange;
      xhr.onload = loadFunc;
      xhr.onerror = errorFunc;
      // IE9 must have onprogress be set to a unique function.
      xhr.onprogress = function () {
      };
      xhr.ontimeout = errorFunc;
      xhr.open(method, uri, !sync);
      //has to be after open
      xhr.withCredentials = !!options.withCredentials;
      // Cannot set timeout with sync request
      // not setting timeout on the xhr object, because of old webkits etc. not handling that correctly
      // both npm's request and jquery 1.x use this kind of timeout, so this is being consistent
      if (!sync && options.timeout > 0) {
        timeoutTimer = setTimeout(function () {
          xhr.abort('timeout')
        }, options.timeout + 2)
      }
      if (xhr.setRequestHeader) {
        for (key in headers) {
          if (headers.hasOwnProperty(key)) {
            xhr.setRequestHeader(key, headers[key])
          }
        }
      } else if (options.headers) {
        throw new Error('Headers cannot be set on an XDomainRequest object')
      }
      if ('responseType' in options) {
        xhr.responseType = options.responseType
      }
      if ('beforeSend' in options && typeof options.beforeSend === 'function') {
        options.beforeSend(xhr)
      }
      xhr.send(body);
      return xhr
    }
    function noop() {
    }
  });
  require.define('crowdstart/node_modules/xhr/node_modules/global/window.js', function (module, exports, __dirname, __filename) {
    'use strict';
    if (typeof window !== 'undefined') {
      module.exports = window
    } else if (typeof global !== 'undefined') {
      module.exports = global
    } else if (typeof self !== 'undefined') {
      module.exports = self
    } else {
      module.exports = {}
    }
  });
  require.define('crowdstart/node_modules/xhr/node_modules/once/once.js', function (module, exports, __dirname, __filename) {
    'use strict';
    module.exports = once;
    once.proto = once(function () {
      Object.defineProperty(Function.prototype, 'once', {
        value: function () {
          return once(this)
        },
        configurable: true
      })
    });
    function once(fn) {
      var called = false;
      return function () {
        if (called)
          return;
        called = true;
        return fn.apply(this, arguments)
      }
    }
  });
  require.define('crowdstart/node_modules/xhr/node_modules/parse-headers/parse-headers.js', function (module, exports, __dirname, __filename) {
    'use strict';
    var trim = require('crowdstart/node_modules/xhr/node_modules/parse-headers/node_modules/trim/index.js'), forEach = require('crowdstart/node_modules/xhr/node_modules/parse-headers/node_modules/for-each/index.js'), isArray = function (arg) {
        return Object.prototype.toString.call(arg) === '[object Array]'
      };
    module.exports = function (headers) {
      if (!headers)
        return {};
      var result = {};
      forEach(trim(headers).split('\n'), function (row) {
        var index = row.indexOf(':'), key = trim(row.slice(0, index)).toLowerCase(), value = trim(row.slice(index + 1));
        if (typeof result[key] === 'undefined') {
          result[key] = value
        } else if (isArray(result[key])) {
          result[key].push(value)
        } else {
          result[key] = [
            result[key],
            value
          ]
        }
      });
      return result
    }
  });
  require.define('crowdstart/node_modules/xhr/node_modules/parse-headers/node_modules/trim/index.js', function (module, exports, __dirname, __filename) {
    'use strict';
    exports = module.exports = trim;
    function trim(str) {
      return str.replace(/^\s*|\s*$/g, '')
    }
    exports.left = function (str) {
      return str.replace(/^\s*/, '')
    };
    exports.right = function (str) {
      return str.replace(/\s*$/, '')
    }
  });
  require.define('crowdstart/node_modules/xhr/node_modules/parse-headers/node_modules/for-each/index.js', function (module, exports, __dirname, __filename) {
    'use strict';
    var isFunction = require('crowdstart/node_modules/xhr/node_modules/parse-headers/node_modules/for-each/node_modules/is-function/index.js');
    module.exports = forEach;
    var toString = Object.prototype.toString;
    var hasOwnProperty = Object.prototype.hasOwnProperty;
    function forEach(list, iterator, context) {
      if (!isFunction(iterator)) {
        throw new TypeError('iterator must be a function')
      }
      if (arguments.length < 3) {
        context = this
      }
      if (toString.call(list) === '[object Array]')
        forEachArray(list, iterator, context);
      else if (typeof list === 'string')
        forEachString(list, iterator, context);
      else
        forEachObject(list, iterator, context)
    }
    function forEachArray(array, iterator, context) {
      for (var i = 0, len = array.length; i < len; i++) {
        if (hasOwnProperty.call(array, i)) {
          iterator.call(context, array[i], i, array)
        }
      }
    }
    function forEachString(string, iterator, context) {
      for (var i = 0, len = string.length; i < len; i++) {
        // no such thing as a sparse string.
        iterator.call(context, string.charAt(i), i, string)
      }
    }
    function forEachObject(object, iterator, context) {
      for (var k in object) {
        if (hasOwnProperty.call(object, k)) {
          iterator.call(context, object[k], k, object)
        }
      }
    }
  });
  require.define('crowdstart/node_modules/xhr/node_modules/parse-headers/node_modules/for-each/node_modules/is-function/index.js', function (module, exports, __dirname, __filename) {
    'use strict';
    module.exports = isFunction;
    var toString = Object.prototype.toString;
    function isFunction(fn) {
      var string = toString.call(fn);
      return string === '[object Function]' || typeof fn === 'function' && string !== '[object RegExp]' || typeof window !== 'undefined' && // IE8 and below
      (fn === window.setTimeout || fn === window.alert || fn === window.confirm || fn === window.prompt)
    }
    ;
  });
  require.define('./api', function (module, exports, __dirname, __filename) {
    'use strict';
    var Crowdstart;
    Crowdstart = require('crowdstart.js/src');
    $('.charge').click(function (e) {
      return Crowdstart.charge({
        payment: {
          type: 'stripe',
          account: {
            number: '4242424242424242',
            month: '12',
            year: '2016',
            cvc: '123'
          },
          metadata: { paid: 'in full' }
        },
        user: {
          email: 'suchfan@shirtlessinseattle.com',
          firstName: 'Sam',
          LastName: 'Ryan',
          company: 'Peabody Conservatory of Music',
          phone: '555-555-5555',
          address: {
            line1: '12345 Faux Road',
            city: 'Seattle',
            state: 'Washington',
            country: 'United States',
            postalCode: '55555-5555'
          },
          metadata: { sleepless: true }
        },
        order: {
          currency: 'usd',
          items: [{
              productId: '1',
              variantId: '1',
              collectionId: '1',
              price: 100,
              quantity: 20
            }],
          metadata: { shippingNotes: 'Ship Ship to da moon.' }
        }
      }, function (status, data) {
        return console.log(status, data)
      })
    })
  });
  require('./api')
}.call(this, this))//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIm5vZGVfbW9kdWxlcy9jcm93ZHN0YXJ0LmpzL3NyYy9pbmRleC5jb2ZmZWUiLCJub2RlX21vZHVsZXMvY3Jvd2RzdGFydC5qcy9ub2RlX21vZHVsZXMveGhyL2luZGV4LmpzIiwibm9kZV9tb2R1bGVzL2Nyb3dkc3RhcnQuanMvbm9kZV9tb2R1bGVzL3hoci9ub2RlX21vZHVsZXMvZ2xvYmFsL3dpbmRvdy5qcyIsIm5vZGVfbW9kdWxlcy9jcm93ZHN0YXJ0LmpzL25vZGVfbW9kdWxlcy94aHIvbm9kZV9tb2R1bGVzL29uY2Uvb25jZS5qcyIsIm5vZGVfbW9kdWxlcy9jcm93ZHN0YXJ0LmpzL25vZGVfbW9kdWxlcy94aHIvbm9kZV9tb2R1bGVzL3BhcnNlLWhlYWRlcnMvcGFyc2UtaGVhZGVycy5qcyIsIm5vZGVfbW9kdWxlcy9jcm93ZHN0YXJ0LmpzL25vZGVfbW9kdWxlcy94aHIvbm9kZV9tb2R1bGVzL3BhcnNlLWhlYWRlcnMvbm9kZV9tb2R1bGVzL3RyaW0vaW5kZXguanMiLCJub2RlX21vZHVsZXMvY3Jvd2RzdGFydC5qcy9ub2RlX21vZHVsZXMveGhyL25vZGVfbW9kdWxlcy9wYXJzZS1oZWFkZXJzL25vZGVfbW9kdWxlcy9mb3ItZWFjaC9pbmRleC5qcyIsIm5vZGVfbW9kdWxlcy9jcm93ZHN0YXJ0LmpzL25vZGVfbW9kdWxlcy94aHIvbm9kZV9tb2R1bGVzL3BhcnNlLWhlYWRlcnMvbm9kZV9tb2R1bGVzL2Zvci1lYWNoL25vZGVfbW9kdWxlcy9pcy1mdW5jdGlvbi9pbmRleC5qcyIsImFwaS5jb2ZmZWUiXSwibmFtZXMiOlsiQ3Jvd2RzdGFydCIsImdsb2JhbCIsInhociIsInJlcXVpcmUiLCJwcm90b3R5cGUiLCJlbmRwb2ludCIsImtleSIsInNldEtleSIsInJlcSIsInVyaSIsImRhdGEiLCJjYiIsIm1ldGhvZCIsImhlYWRlcnMiLCJib2R5IiwiSlNPTiIsInN0cmluZ2lmeSIsImVyciIsInJlcyIsInN0YXR1cyIsInBhcnNlIiwiYXV0aG9yaXplIiwiY2hhcmdlIiwid2luZG93IiwibW9kdWxlIiwiZXhwb3J0cyIsIm9uY2UiLCJwYXJzZUhlYWRlcnMiLCJYSFIiLCJYTUxIdHRwUmVxdWVzdCIsIm5vb3AiLCJYRFIiLCJYRG9tYWluUmVxdWVzdCIsImNyZWF0ZVhIUiIsIm9wdGlvbnMiLCJjYWxsYmFjayIsInJlYWR5c3RhdGVjaGFuZ2UiLCJyZWFkeVN0YXRlIiwibG9hZEZ1bmMiLCJnZXRCb2R5IiwidW5kZWZpbmVkIiwicmVzcG9uc2UiLCJyZXNwb25zZVR5cGUiLCJyZXNwb25zZVRleHQiLCJyZXNwb25zZVhNTCIsImlzSnNvbiIsImUiLCJmYWlsdXJlUmVzcG9uc2UiLCJzdGF0dXNDb2RlIiwidXJsIiwicmF3UmVxdWVzdCIsImVycm9yRnVuYyIsImV2dCIsImNsZWFyVGltZW91dCIsInRpbWVvdXRUaW1lciIsIkVycm9yIiwiZ2V0QWxsUmVzcG9uc2VIZWFkZXJzIiwiY29ycyIsInVzZVhEUiIsInN5bmMiLCJqc29uIiwib25yZWFkeXN0YXRlY2hhbmdlIiwib25sb2FkIiwib25lcnJvciIsIm9ucHJvZ3Jlc3MiLCJvbnRpbWVvdXQiLCJvcGVuIiwid2l0aENyZWRlbnRpYWxzIiwidGltZW91dCIsInNldFRpbWVvdXQiLCJhYm9ydCIsInNldFJlcXVlc3RIZWFkZXIiLCJoYXNPd25Qcm9wZXJ0eSIsImJlZm9yZVNlbmQiLCJzZW5kIiwic2VsZiIsInByb3RvIiwiT2JqZWN0IiwiZGVmaW5lUHJvcGVydHkiLCJGdW5jdGlvbiIsInZhbHVlIiwiY29uZmlndXJhYmxlIiwiZm4iLCJjYWxsZWQiLCJhcHBseSIsImFyZ3VtZW50cyIsInRyaW0iLCJmb3JFYWNoIiwiaXNBcnJheSIsImFyZyIsInRvU3RyaW5nIiwiY2FsbCIsInJlc3VsdCIsInNwbGl0Iiwicm93IiwiaW5kZXgiLCJpbmRleE9mIiwic2xpY2UiLCJ0b0xvd2VyQ2FzZSIsInB1c2giLCJzdHIiLCJyZXBsYWNlIiwibGVmdCIsInJpZ2h0IiwiaXNGdW5jdGlvbiIsImxpc3QiLCJpdGVyYXRvciIsImNvbnRleHQiLCJUeXBlRXJyb3IiLCJsZW5ndGgiLCJmb3JFYWNoQXJyYXkiLCJmb3JFYWNoU3RyaW5nIiwiZm9yRWFjaE9iamVjdCIsImFycmF5IiwiaSIsImxlbiIsInN0cmluZyIsImNoYXJBdCIsIm9iamVjdCIsImsiLCJhbGVydCIsImNvbmZpcm0iLCJwcm9tcHQiLCIkIiwiY2xpY2siLCJwYXltZW50IiwidHlwZSIsImFjY291bnQiLCJudW1iZXIiLCJtb250aCIsInllYXIiLCJjdmMiLCJtZXRhZGF0YSIsInBhaWQiLCJ1c2VyIiwiZW1haWwiLCJmaXJzdE5hbWUiLCJMYXN0TmFtZSIsImNvbXBhbnkiLCJwaG9uZSIsImFkZHJlc3MiLCJsaW5lMSIsImNpdHkiLCJzdGF0ZSIsImNvdW50cnkiLCJwb3N0YWxDb2RlIiwic2xlZXBsZXNzIiwib3JkZXIiLCJjdXJyZW5jeSIsIml0ZW1zIiwicHJvZHVjdElkIiwidmFyaWFudElkIiwiY29sbGVjdGlvbklkIiwicHJpY2UiLCJxdWFudGl0eSIsInNoaXBwaW5nTm90ZXMiLCJjb25zb2xlIiwibG9nIl0sIm1hcHBpbmdzIjoiOzs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7SUFBQSxJQUFJQSxVQUFKLEVBQWdCQyxNQUFoQixFQUF3QkMsR0FBeEIsQztJQUVBQSxHQUFBLEdBQU1DLE9BQUEsQ0FBUSxzQ0FBUixDQUFOLEM7SUFFQUgsVUFBQSxHQUFhLFlBQVk7QUFBQSxNQUN2QkEsVUFBQSxDQUFXSSxTQUFYLENBQXFCQyxRQUFyQixHQUFnQyw0QkFBaEMsQ0FEdUI7QUFBQSxNQUd2QixTQUFTTCxVQUFULENBQW9CTSxHQUFwQixFQUF5QjtBQUFBLFFBQ3ZCLEtBQUtBLEdBQUwsR0FBV0EsR0FEWTtBQUFBLE9BSEY7QUFBQSxNQU92Qk4sVUFBQSxDQUFXSSxTQUFYLENBQXFCRyxNQUFyQixHQUE4QixVQUFTRCxHQUFULEVBQWM7QUFBQSxRQUMxQyxPQUFPLEtBQUtBLEdBQUws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LEM7SUFBQUEsVUFBQSxHQUFhRyxPQUFBLENBQVEsbUJBQVIsQ0FBYixDO0lBQUF3SCxDQUFBLENBRUUsU0FGRixFQUVhQyxLQUZiLENBRW1CLFVBQUM5RSxDQUFEO0FBQUEsTSxPQUNqQjlDLFVBQUEsQ0FBV3NCLE1BQVgsQ0FDRTtBQUFBLFFBQUF1RyxPQUFBLEVBQ0U7QUFBQSxVQUFBQyxJQUFBLEVBQU0sUUFBTjtBQUFBLFVBQ0FDLE9BQUEsRUFDRTtBQUFBLFlBQUFDLE1BQUEsRUFBUSxrQkFBUjtBQUFBLFlBQ0FDLEtBQUEsRUFBUSxJQURSO0FBQUEsWUFFQUMsSUFBQSxFQUFRLE1BRlI7QUFBQSxZQUdBQyxHQUFBLEVBQVEsS0FIUjtBQUFBLFdBRkY7QUFBQSxVQU1BQyxRQUFBLEVBQ0UsRUFBQUMsSUFBQSxFQUFNLFNBQU4sRUFQRjtBQUFBLFNBREY7QUFBQSxRQVVBQyxJQUFBLEVBQ0k7QUFBQSxVQUFBQyxLQUFBLEVBQVcsZ0NBQVg7QUFBQSxVQUNBQyxTQUFBLEVBQVcsS0FEWDtBQUFBLFVBRUFDLFFBQUEsRUFBVyxNQUZYO0FBQUEsVUFHQUMsT0FBQSxFQUFXLCtCQUhYO0FBQUEsVUFJQUMsS0FBQSxFQUFXLGNBSlg7QUFBQSxVQUtBQyxPQUFBLEVBQ0U7QUFBQSxZQUFBQyxLQUFBLEVBQVksaUJBQVo7QUFBQSxZQUNBQyxJQUFBLEVBQVksU0FEWjtBQUFBLFlBRUFDLEtBQUEsRUFBWSxZQUZaO0FBQUEsWUFHQUMsT0FBQSxFQUFZLGVBSFo7QUFBQSxZQUlBQyxVQUFBLEVBQVksWUFKWjtBQUFBLFdBTkY7QUFBQSxVQVdBYixRQUFBLEVBQ0UsRUFBQWMsU0FBQSxFQUFXLElBQVgsRUFaRjtBQUFBLFNBWEo7QUFBQSxRQXlCQUMsS0FBQSxFQUNFO0FBQUEsVUFBQUMsUUFBQSxFQUFVLEtBQVY7QUFBQSxVQUNBQyxLQUFBLEVBQU8sQ0FDTDtBQUFBLGNBQUFDLFNBQUEsRUFBYyxHQUFkO0FBQUEsY0FDQUMsU0FBQSxFQUFjLEdBRGQ7QUFBQSxjQUVBQyxZQUFBLEVBQWMsR0FGZDtBQUFBLGNBR0FDLEtBQUEsRUFBYyxHQUhkO0FBQUEsY0FJQUMsUUFBQSxFQUFjLEVBSmQ7QUFBQSxhQURLLENBRFA7QUFBQSxVQVFBdEIsUUFBQSxFQUNFLEVBQUF1QixhQUFBLEVBQWUsdUJBQWYsRUFURjtBQUFBLFNBMUJGO0FBQUEsT0FERixFQXFDRSxVQUFDeEksTUFBRCxFQUFTVCxJQUFUO0FBQUEsUSxPQUNBa0osT0FBQSxDQUFRQyxHQUFSLENBQVkxSSxNQUFaLEVBQW9CVCxJQUFwQixDQURBO0FBQUEsT0FyQ0YsQ0FEaUI7QUFBQSxLQUZuQixDIiwic291cmNlUm9vdCI6Ii9hc3NldHMvanMvYXBpIn0=