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
  require.define('./index', function (module, exports, __dirname, __filename) {
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
      Crowdstart.prototype.req = function (uri, payload, cb) {
        return xhr({
          body: JSON.stringify(payload),
          uri: this.endpoint + uri,
          headers: {
            'Content-Type': 'application/json',
            'Authorization': this.key
          }
        }, function (err, res, body) {
          return cb(status, JSON.parse(body))
        })
      };
      Crowdstart.prototype.authorize = function (data, cb) {
        return this.req('/authorize', cb)
      };
      Crowdstart.prototype.charge = function (data, cb) {
        return this.req('/charge', cb)
      };
      return Crowdstart
    }();
    if (typeof window !== 'undefined') {
      global = window
    }
    global.Crowdstart = new Crowdstart
  });
  require('./index')
}.call(this, this))//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJzb3VyY2VzIjpbIm5vZGVfbW9kdWxlcy9jcm93ZHN0YXJ0LmpzL25vZGVfbW9kdWxlcy94aHIvaW5kZXguanMiLCJub2RlX21vZHVsZXMvY3Jvd2RzdGFydC5qcy9ub2RlX21vZHVsZXMveGhyL25vZGVfbW9kdWxlcy9nbG9iYWwvd2luZG93LmpzIiwibm9kZV9tb2R1bGVzL2Nyb3dkc3RhcnQuanMvbm9kZV9tb2R1bGVzL3hoci9ub2RlX21vZHVsZXMvb25jZS9vbmNlLmpzIiwibm9kZV9tb2R1bGVzL2Nyb3dkc3RhcnQuanMvbm9kZV9tb2R1bGVzL3hoci9ub2RlX21vZHVsZXMvcGFyc2UtaGVhZGVycy9wYXJzZS1oZWFkZXJzLmpzIiwibm9kZV9tb2R1bGVzL2Nyb3dkc3RhcnQuanMvbm9kZV9tb2R1bGVzL3hoci9ub2RlX21vZHVsZXMvcGFyc2UtaGVhZGVycy9ub2RlX21vZHVsZXMvdHJpbS9pbmRleC5qcyIsIm5vZGVfbW9kdWxlcy9jcm93ZHN0YXJ0LmpzL25vZGVfbW9kdWxlcy94aHIvbm9kZV9tb2R1bGVzL3BhcnNlLWhlYWRlcnMvbm9kZV9tb2R1bGVzL2Zvci1lYWNoL2luZGV4LmpzIiwibm9kZV9tb2R1bGVzL2Nyb3dkc3RhcnQuanMvbm9kZV9tb2R1bGVzL3hoci9ub2RlX21vZHVsZXMvcGFyc2UtaGVhZGVycy9ub2RlX21vZHVsZXMvZm9yLWVhY2gvbm9kZV9tb2R1bGVzL2lzLWZ1bmN0aW9uL2luZGV4LmpzIiwiaW5kZXguY29mZmVlIl0sIm5hbWVzIjpbIndpbmRvdyIsInJlcXVpcmUiLCJvbmNlIiwicGFyc2VIZWFkZXJzIiwiWEhSIiwiWE1MSHR0cFJlcXVlc3QiLCJub29wIiwiWERSIiwiWERvbWFpblJlcXVlc3QiLCJtb2R1bGUiLCJleHBvcnRzIiwiY3JlYXRlWEhSIiwib3B0aW9ucyIsImNhbGxiYWNrIiwicmVhZHlzdGF0ZWNoYW5nZSIsInhociIsInJlYWR5U3RhdGUiLCJsb2FkRnVuYyIsImdldEJvZHkiLCJib2R5IiwidW5kZWZpbmVkIiwicmVzcG9uc2UiLCJyZXNwb25zZVR5cGUiLCJyZXNwb25zZVRleHQiLCJyZXNwb25zZVhNTCIsImlzSnNvbiIsIkpTT04iLCJwYXJzZSIsImUiLCJmYWlsdXJlUmVzcG9uc2UiLCJoZWFkZXJzIiwic3RhdHVzQ29kZSIsIm1ldGhvZCIsInVybCIsInVyaSIsInJhd1JlcXVlc3QiLCJlcnJvckZ1bmMiLCJldnQiLCJjbGVhclRpbWVvdXQiLCJ0aW1lb3V0VGltZXIiLCJFcnJvciIsInN0YXR1cyIsImVyciIsImdldEFsbFJlc3BvbnNlSGVhZGVycyIsImNvcnMiLCJ1c2VYRFIiLCJrZXkiLCJkYXRhIiwic3luYyIsInN0cmluZ2lmeSIsImpzb24iLCJvbnJlYWR5c3RhdGVjaGFuZ2UiLCJvbmxvYWQiLCJvbmVycm9yIiwib25wcm9ncmVzcyIsIm9udGltZW91dCIsIm9wZW4iLCJ3aXRoQ3JlZGVudGlhbHMiLCJ0aW1lb3V0Iiwic2V0VGltZW91dCIsImFib3J0Iiwic2V0UmVxdWVzdEhlYWRlciIsImhhc093blByb3BlcnR5IiwiYmVmb3JlU2VuZCIsInNlbmQiLCJnbG9iYWwiLCJzZWxmIiwicHJvdG8iLCJPYmplY3QiLCJkZWZpbmVQcm9wZXJ0eSIsIkZ1bmN0aW9uIiwicHJvdG90eXBlIiwidmFsdWUiLCJjb25maWd1cmFibGUiLCJmbiIsImNhbGxlZCIsImFwcGx5IiwiYXJndW1lbnRzIiwidHJpbSIsImZvckVhY2giLCJpc0FycmF5IiwiYXJnIiwidG9TdHJpbmciLCJjYWxsIiwicmVzdWx0Iiwic3BsaXQiLCJyb3ciLCJpbmRleCIsImluZGV4T2YiLCJzbGljZSIsInRvTG93ZXJDYXNlIiwicHVzaCIsInN0ciIsInJlcGxhY2UiLCJsZWZ0IiwicmlnaHQiLCJpc0Z1bmN0aW9uIiwibGlzdCIsIml0ZXJhdG9yIiwiY29udGV4dCIsIlR5cGVFcnJvciIsImxlbmd0aCIsImZvckVhY2hBcnJheSIsImZvckVhY2hTdHJpbmciLCJmb3JFYWNoT2JqZWN0IiwiYXJyYXkiLCJpIiwibGVuIiwic3RyaW5nIiwiY2hhckF0Iiwib2JqZWN0IiwiayIsImFsZXJ0IiwiY29uZmlybSIsInByb21wdCIsIkNyb3dkc3RhcnQiLCJlbmRwb2ludCIsInNldEtleSIsInJlcSIsInBheWxvYWQiLCJjYiIsInJlcyIsImF1dGhvcml6ZSIsImNoYXJnZSJdLCJtYXBwaW5ncyI6Ijs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7Ozs7O0lBQUEsYTtJQUNBLElBQUlBLE1BQUEsR0FBU0MsT0FBQSxDQUFRLDJEQUFSLENBQWIsQztJQUNBLElBQUlDLElBQUEsR0FBT0QsT0FBQSxDQUFRLHVEQUFSLENBQVgsQztJQUNBLElBQUlFLFlBQUEsR0FBZUYsT0FBQSxDQUFRLHlFQUFSLENBQW5CLEM7SUFHQSxJQUFJRyxHQUFBLEdBQU1KLE1BQUEsQ0FBT0ssY0FBUCxJQUF5QkMsSUFBbkMsQztJQUNBLElBQUlDLEdBQUEsR0FBTSxxQkFBcUIsSUFBS0gsR0FBMUIsR0FBbUNBLEdBQW5DLEdBQXlDSixNQUFBLENBQU9RLGNBQTFELEM7SUFFQUMsTUFBQSxDQUFPQyxPQUFQLEdBQWlCQyxTQUFqQixDO0lBRUEsU0FBU0EsU0FBVCxDQUFtQkMsT0FBbkIsRUFBNEJDLFFBQTVCLEVBQXNDO0FBQUEsTUFDbEMsU0FBU0MsZ0JBQVQsR0FBNEI7QUFBQSxRQUN4QixJQUFJQyxHQUFBLENBQUlDLFVBQUosS0FBbUIsQ0FBdkIsRUFBMEI7QUFBQSxVQUN0QkMsUUFBQSxFQURzQjtBQUFBLFNBREY7QUFBQSxPQURNO0FBQUEsTUFPbEMsU0FBU0MsT0FBVCxHQUFtQjtBQUFBLFFBRWY7QUFBQSxZQUFJQyxJQUFBLEdBQU9DLFNBQVgsQ0FGZTtBQUFBLFFBSWYsSUFBSUwsR0FBQSxDQUFJTSxRQUFSLEVBQWtCO0FBQUEsVUFDZEYsSUFBQSxHQUFPSixHQUFBLENBQUlNLFFBREc7QUFBQSxTQUFsQixNQUVPLElBQUlOLEdBQUEsQ0FBSU8sWUFBSixLQUFxQixNQUFyQixJQUErQixDQUFDUCxHQUFBLENBQUlP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QyxVQUFBLEVBQUFsRCxNQUFBLEVBQUFsRCxHQUFBLEM7SUFBQUEsR0FBQSxHQUFNZCxPQUFBLENBQVEsc0NBQVIsQ0FBTixDO0lBQUFrSCxVQUFBLEc7TUFHRUEsVUFBQSxDQUFBNUMsU0FBQSxDQUFBNkMsUUFBQSxHQUFVLDRCQUFWLEM7TUFDYSxTQUFBRCxVQUFBLENBQUVyRSxHQUFGO0FBQUEsUUFBQyxLQUFDQSxHQUFELEdBQUNBLEdBQUY7QUFBQSxPO01BRGJxRSxVQUFBLENBQUE1QyxTQUFBLENBR0E4QyxNQUhBLEdBR1EsVUFBQ3ZFLEdBQUQ7QUFBQSxRLE9BQ04sS0FBQ0EsR0FBRCxHQUFPQSxHQUREO0FBQUEsT0FIUixDO01BQUFxRSxVQUFBLENBQUE1QyxTQUFBLENBTUErQyxHQU5BLEdBTUssVUFBQ3BGLEdBQUQsRUFBTXFGLE9BQU4sRUFBZUMsRUFBZjtBQUFBLFEsT0FDSHpHLEdBQUEsQ0FDRTtBQUFBLFVBQUFJLElBQUEsRUFBTU8sSUFBQSxDQUFLdUIsU0FBTCxDQUFlc0UsT0FBZixDQUFOO0FBQUEsVUFDQXJGLEdBQUEsRUFBTSxLQUFDa0YsUUFBRCxHQUFZbEYsR0FEbEI7QUFBQSxVQUVBSixPQUFBLEVBQ0U7QUFBQSw0QkFBZ0Isa0JBQWhCO0FBQUEsWUFDQSxpQkFBaUIsS0FBQ2dCLEdBRGxCO0FBQUEsV0FIRjtBQUFBLFNBREYsRUFNRSxVQUFDSixHQUFELEVBQU0rRSxHQUFOLEVBQVd0RyxJQUFYO0FBQUEsVSxPQUNBcUcsRUFBQSxDQUFHL0UsTUFBSCxFQUFXZixJQUFBLENBQUtDLEtBQUwsQ0FBV1IsSUFBWCxDQUFYLENBREE7QUFBQSxTQU5GLENBREc7QUFBQSxPQU5MLEM7TUFBQWdHLFVBQUEsQ0FBQTVDLFNBQUEsQ0FnQkFtRCxTQWhCQSxHQWdCVyxVQUFDM0UsSUFBRCxFQUFPeUUsRUFBUDtBQUFBLFEsT0FDVCxLQUFDRixHQUFELENBQUssWUFBTCxFQUFtQkUsRUFBbkIsQ0FEUztBQUFBLE9BaEJYLEM7TUFBQUwsVUFBQSxDQUFBNUMsU0FBQSxDQW1CQW9ELE1BbkJBLEdBbUJRLFVBQUM1RSxJQUFELEVBQU95RSxFQUFQO0FBQUEsUSxPQUNOLEtBQUNGLEdBQUQsQ0FBSyxTQUFMLEVBQWdCRSxFQUFoQixDQURNO0FBQUEsT0FuQlIsQzs7S0FIRixHO0lBeUJBLElBQU8sT0FBQXhILE1BQUEsS0FBaUIsV0FBeEI7QUFBQSxNQUNFaUUsTUFBQSxHQUFTakUsTUFEWDtBQUFBLEs7SUF6QkFpRSxNQUFBLENBNEJPa0QsVUE1QlAsR0E0QnFCLElBQUFBLFUiLCJzb3VyY2VSb290IjoiL25vZGVfbW9kdWxlcy9jcm93ZHN0YXJ0LmpzL3NyYyJ9