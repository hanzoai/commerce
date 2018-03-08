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
  // Require a module
  function rqzt(file, callback) {
    if ({}.hasOwnProperty.call(rqzt.cache, file))
      return rqzt.cache[file];
    // Handle async require
    if (typeof callback == 'function') {
      rqzt.load(file, callback);
      return
    }
    var resolved = rqzt.resolve(file);
    if (!resolved)
      throw new Error('Failed to resolve module ' + file);
    var module$ = {
      id: file,
      rqzt: rqzt,
      filename: file,
      exports: {},
      loaded: false,
      parent: null,
      children: []
    };
    var dirname = file.slice(0, file.lastIndexOf('/') + 1);
    rqzt.cache[file] = module$.exports;
    resolved.call(module$.exports, module$, module$.exports, dirname, file);
    module$.loaded = true;
    return rqzt.cache[file] = module$.exports
  }
  rqzt.modules = {};
  rqzt.cache = {};
  rqzt.resolve = function (file) {
    return {}.hasOwnProperty.call(rqzt.modules, file) ? rqzt.modules[file] : void 0
  };
  // Define normal static module
  rqzt.define = function (file, fn) {
    rqzt.modules[file] = fn
  };
  global.require = rqzt;
  // source: node_modules/crowdstart.js/src/index.coffee
  rqzt.define('crowdstart.js/src', function (module, exports, __dirname, __filename, process) {
    var Crowdstart;
    Crowdstart = new (rqzt('crowdstart.js/src/crowdstart'));
    if (typeof window !== 'undefined') {
      window.Crowdstart = Crowdstart
    } else {
      module.exports = Crowdstart
    }
  });
  // source: node_modules/crowdstart.js/src/crowdstart.coffee
  rqzt.define('crowdstart.js/src/crowdstart', function (module, exports, __dirname, __filename, process) {
    var Crowdstart, xhr;
    xhr = rqzt('xhr');
    Crowdstart = function () {
      Crowdstart.prototype.endpoint = 'https://api.crowdstart.com';
      function Crowdstart(key1) {
        this.key = key1
      }
      Crowdstart.prototype.setKey = function (key) {
        return this.key = key
      };
      Crowdstart.prototype.setStore = function (id) {
        return this.storeId = id
      };
      Crowdstart.prototype.req = function (uri, data, cb) {
        return xhr({
          uri: this.endpoint.replace(/\/$/, '') + uri,
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': this.key
          },
          json: data
        }, function (err, res, body) {
          return cb(res.statusCode, body, res.headers.location)
        })
      };
      Crowdstart.prototype.authorize = function (data, cb) {
        var uri;
        uri = '/authorize';
        if (this.storeId != null) {
          uri = '/store/' + this.storeId + uri
        }
        return this.req('/authorize', data, cb)
      };
      Crowdstart.prototype.charge = function (data, cb) {
        var uri;
        uri = '/charge';
        if (this.storeId != null) {
          uri = '/store/' + this.storeId + uri
        }
        return this.req('/charge', data, cb)
      };
      return Crowdstart
    }();
    module.exports = Crowdstart
  });
  // source: node_modules/xhr/index.js
  rqzt.define('xhr', function (module, exports, __dirname, __filename, process) {
    'use strict';
    var window = rqzt('global/window');
    var isFunction = rqzt('is-function');
    var parseHeaders = rqzt('parse-headers/parse-headers');
    var xtend = rqzt('xtend/immutable');
    module.exports = createXHR;
    createXHR.XMLHttpRequest = window.XMLHttpRequest || noop;
    createXHR.XDomainRequest = 'withCredentials' in new createXHR.XMLHttpRequest ? createXHR.XMLHttpRequest : window.XDomainRequest;
    forEachArray([
      'get',
      'put',
      'post',
      'patch',
      'head',
      'delete'
    ], function (method) {
      createXHR[method === 'delete' ? 'del' : method] = function (uri, options, callback) {
        options = initParams(uri, options, callback);
        options.method = method.toUpperCase();
        return _createXHR(options)
      }
    });
    function forEachArray(array, iterator) {
      for (var i = 0; i < array.length; i++) {
        iterator(array[i])
      }
    }
    function isEmpty(obj) {
      for (var i in obj) {
        if (obj.hasOwnProperty(i))
          return false
      }
      return true
    }
    function initParams(uri, options, callback) {
      var params = uri;
      if (isFunction(options)) {
        callback = options;
        if (typeof uri === 'string') {
          params = { uri: uri }
        }
      } else {
        params = xtend(options, { uri: uri })
      }
      params.callback = callback;
      return params
    }
    function createXHR(uri, options, callback) {
      options = initParams(uri, options, callback);
      return _createXHR(options)
    }
    function _createXHR(options) {
      if (typeof options.callback === 'undefined') {
        throw new Error('callback argument missing')
      }
      var called = false;
      var callback = function cbOnce(err, response, body) {
        if (!called) {
          called = true;
          options.callback(err, response, body)
        }
      };
      function readystatechange() {
        if (xhr.readyState === 4) {
          setTimeout(loadFunc, 0)
        }
      }
      function getBody() {
        // Chrome with requestType=blob throws errors arround when even testing access to responseText
        var body = undefined;
        if (xhr.response) {
          body = xhr.response
        } else {
          body = xhr.responseText || getXml(xhr)
        }
        if (isJson) {
          try {
            body = JSON.parse(body)
          } catch (e) {
          }
        }
        return body
      }
      function errorFunc(evt) {
        clearTimeout(timeoutTimer);
        if (!(evt instanceof Error)) {
          evt = new Error('' + (evt || 'Unknown XMLHttpRequest Error'))
        }
        evt.statusCode = 0;
        return callback(evt, failureResponse)
      }
      // will load the data & process the response in a special response object
      function loadFunc() {
        if (aborted)
          return;
        var status;
        clearTimeout(timeoutTimer);
        if (options.useXDR && xhr.status === undefined) {
          //IE8 CORS GET successful response doesn't have a status field, but body is fine
          status = 200
        } else {
          status = xhr.status === 1223 ? 204 : xhr.status
        }
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
        return callback(err, response, response.body)
      }
      var xhr = options.xhr || null;
      if (!xhr) {
        if (options.cors || options.useXDR) {
          xhr = new createXHR.XDomainRequest
        } else {
          xhr = new createXHR.XMLHttpRequest
        }
      }
      var key;
      var aborted;
      var uri = xhr.url = options.uri || options.url;
      var method = xhr.method = options.method || 'GET';
      var body = options.body || options.data;
      var headers = xhr.headers = options.headers || {};
      var sync = !!options.sync;
      var isJson = false;
      var timeoutTimer;
      var failureResponse = {
        body: undefined,
        headers: {},
        statusCode: 0,
        method: method,
        url: uri,
        rawRequest: xhr
      };
      if ('json' in options && options.json !== false) {
        isJson = true;
        headers['accept'] || headers['Accept'] || (headers['Accept'] = 'application/json');
        //Don't override existing accept header declared by user
        if (method !== 'GET' && method !== 'HEAD') {
          headers['content-type'] || headers['Content-Type'] || (headers['Content-Type'] = 'application/json');
          //Don't override existing accept header declared by user
          body = JSON.stringify(options.json === true ? body : options.json)
        }
      }
      xhr.onreadystatechange = readystatechange;
      xhr.onload = loadFunc;
      xhr.onerror = errorFunc;
      // IE9 must have onprogress be set to a unique function.
      xhr.onprogress = function () {
      };
      xhr.onabort = function () {
        aborted = true
      };
      xhr.ontimeout = errorFunc;
      xhr.open(method, uri, !sync, options.username, options.password);
      //has to be after open
      if (!sync) {
        xhr.withCredentials = !!options.withCredentials
      }
      // Cannot set timeout with sync request
      // not setting timeout on the xhr object, because of old webkits etc. not handling that correctly
      // both npm's request and jquery 1.x use this kind of timeout, so this is being consistent
      if (!sync && options.timeout > 0) {
        timeoutTimer = setTimeout(function () {
          if (aborted)
            return;
          aborted = true;
          //IE9 may still call readystatechange
          xhr.abort('timeout');
          var e = new Error('XMLHttpRequest timeout');
          e.code = 'ETIMEDOUT';
          errorFunc(e)
        }, options.timeout)
      }
      if (xhr.setRequestHeader) {
        for (key in headers) {
          if (headers.hasOwnProperty(key)) {
            xhr.setRequestHeader(key, headers[key])
          }
        }
      } else if (options.headers && !isEmpty(options.headers)) {
        throw new Error('Headers cannot be set on an XDomainRequest object')
      }
      if ('responseType' in options) {
        xhr.responseType = options.responseType
      }
      if ('beforeSend' in options && typeof options.beforeSend === 'function') {
        options.beforeSend(xhr)
      }
      // Microsoft Edge browser sends "undefined" when send is called with undefined value.
      // XMLHttpRequest spec says to pass null as body to indicate no body
      // See https://github.com/naugtur/xhr/issues/100.
      xhr.send(body || null);
      return xhr
    }
    function getXml(xhr) {
      // xhr.responseXML will throw Exception "InvalidStateError" or "DOMException"
      // See https://developer.mozilla.org/en-US/docs/Web/API/XMLHttpRequest/responseXML.
      try {
        if (xhr.responseType === 'document') {
          return xhr.responseXML
        }
        var firefoxBugTakenEffect = xhr.responseXML && xhr.responseXML.documentElement.nodeName === 'parsererror';
        if (xhr.responseType === '' && !firefoxBugTakenEffect) {
          return xhr.responseXML
        }
      } catch (e) {
      }
      return null
    }
    function noop() {
    }
  });
  // source: node_modules/global/window.js
  rqzt.define('global/window', function (module, exports, __dirname, __filename, process) {
    var win;
    if (typeof window !== 'undefined') {
      win = window
    } else if (typeof global !== 'undefined') {
      win = global
    } else if (typeof self !== 'undefined') {
      win = self
    } else {
      win = {}
    }
    module.exports = win
  });
  // source: node_modules/is-function/index.js
  rqzt.define('is-function', function (module, exports, __dirname, __filename, process) {
    module.exports = isFunction;
    var toString = Object.prototype.toString;
    function isFunction(fn) {
      var string = toString.call(fn);
      return string === '[object Function]' || typeof fn === 'function' && string !== '[object RegExp]' || typeof window !== 'undefined' && (fn === window.setTimeout || fn === window.alert || fn === window.confirm || fn === window.prompt)
    }
    ;
  });
  // source: node_modules/parse-headers/parse-headers.js
  rqzt.define('parse-headers/parse-headers', function (module, exports, __dirname, __filename, process) {
    var trim = rqzt('trim'), forEach = rqzt('for-each'), isArray = function (arg) {
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
  // source: node_modules/trim/index.js
  rqzt.define('trim', function (module, exports, __dirname, __filename, process) {
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
  // source: node_modules/for-each/index.js
  rqzt.define('for-each', function (module, exports, __dirname, __filename, process) {
    var isFunction = rqzt('is-function');
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
  // source: node_modules/xtend/immutable.js
  rqzt.define('xtend/immutable', function (module, exports, __dirname, __filename, process) {
    module.exports = extend;
    var hasOwnProperty = Object.prototype.hasOwnProperty;
    function extend() {
      var target = {};
      for (var i = 0; i < arguments.length; i++) {
        var source = arguments[i];
        for (var key in source) {
          if (hasOwnProperty.call(source, key)) {
            target[key] = source[key]
          }
        }
      }
      return target
    }
  });
  // source: assets/js/api/order.coffee
  rqzt.define('./order', function (module, exports, __dirname, __filename, process) {
    module.exports = {
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
        metadata: { sleepless: true }
      },
      order: {
        currency: 'usd',
        billingAddress: {
          line1: '12345 Faux Road',
          city: 'Seattle',
          state: 'Washington',
          country: 'US',
          postalCode: '55555-5555'
        },
        shippingAddress: {
          line1: '12345 Faux Road',
          city: 'Seattle',
          state: 'Washington',
          country: 'US',
          postalCode: '55555-5555'
        },
        items: [{
            productSlug: 'doge-shirt',
            quantity: 2
          }],
        metadata: { shippingNotes: 'Ship Ship to da moon.' }
      }
    }
  });
  // source: assets/js/api/api.coffee
  rqzt.define('./api', function (module, exports, __dirname, __filename, process) {
    var order;
    rqzt('crowdstart.js/src');
    order = rqzt('./order');
    $('.charge').click(function (e) {
      return Crowdstart.charge(order, function (status, data, loc) {
        console.log(status, data);
        if (loc != null) {
          return window.location = loc
        }
      })
    });
    $('.authorize').click(function (e) {
      return Crowdstart.authorize(order, function (status, data, loc) {
        console.log(status, data);
        if (loc != null) {
          return window.location = loc
        }
      })
    })
  });
  rqzt('./api')
}.call(this, this))
