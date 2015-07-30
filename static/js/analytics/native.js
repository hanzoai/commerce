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
  // source: /Users/dtai/work/verus/crowdstart/node_modules/espy/src/index.coffee
  require.define('espy/src', function (module, exports, __dirname, __filename) {
    var Espy, cookies, newRecord, qs, sessionIdCookie, store, userAgent, userIdCookie, uuid;
    Espy = function () {
    };
    if (typeof window !== 'undefined' && window !== null) {
      if (window.console == null || window.console.log == null) {
        window.console.log = function () {
        }
      }
      store = require('espy/node_modules/store/store');
      cookies = require('espy/node_modules/cookies-js/dist/cookies');
      userAgent = require('espy/node_modules/ua-parser-js/src/ua-parser');
      qs = require('espy/node_modules/query-string');
      uuid = require('espy/node_modules/node-uuid/uuid');
      userIdCookie = '__cs-uid';
      sessionIdCookie = '__cs-sid';
      newRecord = {
        pageId: '',
        lastPageId: '',
        pageViewId: '',
        lastPageViewId: '',
        count: 0,
        queue: []
      };
      (function () {
        var cachedDomain, cachedPageId, cachedPageViewId, cachedSessionId, cachedUserId, flush, getDomain, getPageId, getPageViewId, getQueryParams, getRecord, getSessionId, getTimestamp, getUserId, next, refreshSession, saveRecord, updatePage;
        getTimestamp = function () {
          return new Date().getMilliseconds()
        };
        cachedDomain = '';
        getDomain = function () {
          if (!cachedDomain) {
            cachedDomain = document.domain !== 'localhost' ? '.' + document.domain : ''
          }
          return cachedDomain
        };
        getRecord = function () {
          var ref;
          return (ref = store.get(getSessionId())) != null ? ref : newRecord
        };
        saveRecord = function (record) {
          return store.set(getSessionId(), record)
        };
        cachedUserId = '';
        getUserId = function () {
          var userId;
          if (cachedUserId) {
            return cachedUserId
          }
          userId = cookies.get(userIdCookie);
          if (!userId) {
            userId = uuid.v4();
            cookies.set(userIdCookie, userId, { domain: getDomain() })
          }
          cachedUserId = userId;
          return userId
        };
        cachedSessionId = '';
        getSessionId = function () {
          var record, sessionId;
          if (cachedSessionId) {
            return cachedSessionId
          }
          sessionId = cookies.get(sessionIdCookie);
          if (!sessionId) {
            sessionId = getUserId() + '_' + getTimestamp();
            cookies.set(sessionIdCookie, sessionId, {
              domain: getDomain(),
              expires: 1800
            });
            cachedSessionId = sessionId;
            record = getRecord();
            record.count = 0;
            saveRecord(record)
          }
          cachedSessionId = sessionId;
          return sessionId
        };
        refreshSession = function () {
          var sessionId;
          sessionId = cookies.get;
          return cookies.set(sessionIdCookie, sessionId, {
            domain: '.' + document.domain,
            expires: 1800
          })
        };
        cachedPageId = '';
        cachedPageViewId = '';
        getPageId = function () {
          return cachedPageId
        };
        getPageViewId = function () {
          return cachedPageViewId
        };
        getQueryParams = function () {
          return qs.parse(window.location.search || window.location.hash.split('?')[1])
        };
        updatePage = function () {
          var newPageId, record;
          record = getRecord();
          newPageId = window.location.pathname + window.location.hash;
          if (newPageId !== record.pageId) {
            cachedPageId = newPageId;
            cachedPageViewId = cachedPageId + '_' + getTimestamp();
            record = getRecord();
            record.lastPageId = record.pageId;
            record.lastPageViewId = record.pageViewId;
            record.pageId = cachedPageId;
            record.pageViewId = cachedPageViewId;
            saveRecord(record);
            return Espy('PageView', {
              lastPageId: record.lastPageId,
              lastPageViewId: record.lastPageViewId,
              url: window.location.href,
              referrerUrl: document.referrer,
              queryParams: getQueryParams()
            })
          }
        };
        Espy = function (name, data) {
          var record, ua;
          ua = window.navigator.userAgent;
          record = getRecord();
          record.queue.push({
            userId: getUserId(),
            sessionId: getSessionId(),
            pageId: record.pageId,
            pageViewId: record.pageViewId,
            uaString: ua,
            ua: userAgent(ua),
            timestamp: new Date,
            event: name,
            data: data,
            count: record.count
          });
          record.count++;
          saveRecord(record);
          return refreshSession()
        };
        flush = function () {
          var data, record, retry, xhr;
          record = getRecord();
          if (record.queue.length > 0) {
            Espy.onflush(record);
            retry = 0;
            data = JSON.stringify(record.queue);
            xhr = new XMLHttpRequest;
            xhr.onreadystatechange = function () {
              if (xhr.readyState === 4) {
                if (xhr.status !== 204) {
                  retry++;
                  if (retry === 3) {
                    return console.log('Espy: failed to send', JSON.parse(data))
                  } else {
                    xhr.open('POST', Espy.url);
                    xhr.send(data);
                    return console.log('Espy: retrying send x' + retry)
                  }
                }
              }
            };
            xhr.open('POST', Espy.url);
            xhr.setRequestHeader('Content-Type', 'application/json');
            xhr.send(data);
            record.queue.length = 0;
            return saveRecord(record)
          }
        };
        window.addEventListener('hashchange', updatePage);
        window.addEventListener('popstate', updatePage);
        window.addEventListener('beforeunload', function () {
          return Espy('PageChange')
        });
        updatePage();
        next = function () {
          return setTimeout(function () {
            flush();
            return next()
          }, Espy.flushRate || 200)
        };
        setTimeout(function () {
          return next()
        }, 1);
        return window.Espy = Espy
      }())
    }
    Espy.url = 'https://analytics.crowdstart.com/';
    Espy.onflush = function () {
    };
    Espy.flushRate = 200;
    module.exports = Espy
  });
  // source: /Users/dtai/work/verus/crowdstart/node_modules/espy/node_modules/store/store.js
  require.define('espy/node_modules/store/store', function (module, exports, __dirname, __filename) {
    ;
    (function (win) {
      var store = {}, doc = win.document, localStorageName = 'localStorage', scriptTag = 'script', storage;
      store.disabled = false;
      store.version = '1.3.17';
      store.set = function (key, value) {
      };
      store.get = function (key, defaultVal) {
      };
      store.has = function (key) {
        return store.get(key) !== undefined
      };
      store.remove = function (key) {
      };
      store.clear = function () {
      };
      store.transact = function (key, defaultVal, transactionFn) {
        if (transactionFn == null) {
          transactionFn = defaultVal;
          defaultVal = null
        }
        if (defaultVal == null) {
          defaultVal = {}
        }
        var val = store.get(key, defaultVal);
        transactionFn(val);
        store.set(key, val)
      };
      store.getAll = function () {
      };
      store.forEach = function () {
      };
      store.serialize = function (value) {
        return JSON.stringify(value)
      };
      store.deserialize = function (value) {
        if (typeof value != 'string') {
          return undefined
        }
        try {
          return JSON.parse(value)
        } catch (e) {
          return value || undefined
        }
      };
      // Functions to encapsulate questionable FireFox 3.6.13 behavior
      // when about.config::dom.storage.enabled === false
      // See https://github.com/marcuswestin/store.js/issues#issue/13
      function isLocalStorageNameSupported() {
        try {
          return localStorageName in win && win[localStorageName]
        } catch (err) {
          return false
        }
      }
      if (isLocalStorageNameSupported()) {
        storage = win[localStorageName];
        store.set = function (key, val) {
          if (val === undefined) {
            return store.remove(key)
          }
          storage.setItem(key, store.serialize(val));
          return val
        };
        store.get = function (key, defaultVal) {
          var val = store.deserialize(storage.getItem(key));
          return val === undefined ? defaultVal : val
        };
        store.remove = function (key) {
          storage.removeItem(key)
        };
        store.clear = function () {
          storage.clear()
        };
        store.getAll = function () {
          var ret = {};
          store.forEach(function (key, val) {
            ret[key] = val
          });
          return ret
        };
        store.forEach = function (callback) {
          for (var i = 0; i < storage.length; i++) {
            var key = storage.key(i);
            callback(key, store.get(key))
          }
        }
      } else if (doc.documentElement.addBehavior) {
        var storageOwner, storageContainer;
        // Since #userData storage applies only to specific paths, we need to
        // somehow link our data to a specific path.  We choose /favicon.ico
        // as a pretty safe option, since all browsers already make a request to
        // this URL anyway and being a 404 will not hurt us here.  We wrap an
        // iframe pointing to the favicon in an ActiveXObject(htmlfile) object
        // (see: http://msdn.microsoft.com/en-us/library/aa752574(v=VS.85).aspx)
        // since the iframe access rules appear to allow direct access and
        // manipulation of the document element, even for a 404 page.  This
        // document can be used instead of the current document (which would
        // have been limited to the current path) to perform #userData storage.
        try {
          storageContainer = new ActiveXObject('htmlfile');
          storageContainer.open();
          storageContainer.write('<' + scriptTag + '>document.w=window</' + scriptTag + '><iframe src="/favicon.ico"></iframe>');
          storageContainer.close();
          storageOwner = storageContainer.w.frames[0].document;
          storage = storageOwner.createElement('div')
        } catch (e) {
          // somehow ActiveXObject instantiation failed (perhaps some special
          // security settings or otherwse), fall back to per-path storage
          storage = doc.createElement('div');
          storageOwner = doc.body
        }
        var withIEStorage = function (storeFunction) {
          return function () {
            var args = Array.prototype.slice.call(arguments, 0);
            args.unshift(storage);
            // See http://msdn.microsoft.com/en-us/library/ms531081(v=VS.85).aspx
            // and http://msdn.microsoft.com/en-us/library/ms531424(v=VS.85).aspx
            storageOwner.appendChild(storage);
            storage.addBehavior('#default#userData');
            storage.load(localStorageName);
            var result = storeFunction.apply(store, args);
            storageOwner.removeChild(storage);
            return result
          }
        };
        // In IE7, keys cannot start with a digit or contain certain chars.
        // See https://github.com/marcuswestin/store.js/issues/40
        // See https://github.com/marcuswestin/store.js/issues/83
        var forbiddenCharsRegex = new RegExp('[!"#$%&\'()*+,/\\\\:;<=>?@[\\]^`{|}~]', 'g');
        function ieKeyFix(key) {
          return key.replace(/^d/, '___$&').replace(forbiddenCharsRegex, '___')
        }
        store.set = withIEStorage(function (storage, key, val) {
          key = ieKeyFix(key);
          if (val === undefined) {
            return store.remove(key)
          }
          storage.setAttribute(key, store.serialize(val));
          storage.save(localStorageName);
          return val
        });
        store.get = withIEStorage(function (storage, key, defaultVal) {
          key = ieKeyFix(key);
          var val = store.deserialize(storage.getAttribute(key));
          return val === undefined ? defaultVal : val
        });
        store.remove = withIEStorage(function (storage, key) {
          key = ieKeyFix(key);
          storage.removeAttribute(key);
          storage.save(localStorageName)
        });
        store.clear = withIEStorage(function (storage) {
          var attributes = storage.XMLDocument.documentElement.attributes;
          storage.load(localStorageName);
          for (var i = 0, attr; attr = attributes[i]; i++) {
            storage.removeAttribute(attr.name)
          }
          storage.save(localStorageName)
        });
        store.getAll = function (storage) {
          var ret = {};
          store.forEach(function (key, val) {
            ret[key] = val
          });
          return ret
        };
        store.forEach = withIEStorage(function (storage, callback) {
          var attributes = storage.XMLDocument.documentElement.attributes;
          for (var i = 0, attr; attr = attributes[i]; ++i) {
            callback(attr.name, store.deserialize(storage.getAttribute(attr.name)))
          }
        })
      }
      try {
        var testKey = '__storejs__';
        store.set(testKey, testKey);
        if (store.get(testKey) != testKey) {
          store.disabled = true
        }
        store.remove(testKey)
      } catch (e) {
        store.disabled = true
      }
      store.enabled = !store.disabled;
      if (typeof module != 'undefined' && module.exports && this.module !== module) {
        module.exports = store
      } else if (typeof define === 'function' && define.amd) {
        define(store)
      } else {
        win.store = store
      }
    }(Function('return this')()))
  });
  // source: /Users/dtai/work/verus/crowdstart/node_modules/espy/node_modules/cookies-js/dist/cookies.js
  require.define('espy/node_modules/cookies-js/dist/cookies', function (module, exports, __dirname, __filename) {
    /*
 * Cookies.js - 1.2.1
 * https://github.com/ScottHamper/Cookies
 *
 * This is free and unencumbered software released into the public domain.
 */
    (function (global, undefined) {
      'use strict';
      var factory = function (window) {
        if (typeof window.document !== 'object') {
          throw new Error('Cookies.js requires a `window` with a `document` object')
        }
        var Cookies = function (key, value, options) {
          return arguments.length === 1 ? Cookies.get(key) : Cookies.set(key, value, options)
        };
        // Allows for setter injection in unit tests
        Cookies._document = window.document;
        // Used to ensure cookie keys do not collide with
        // built-in `Object` properties
        Cookies._cacheKeyPrefix = 'cookey.';
        // Hurr hurr, :)
        Cookies._maxExpireDate = new Date('Fri, 31 Dec 9999 23:59:59 UTC');
        Cookies.defaults = {
          path: '/',
          secure: false
        };
        Cookies.get = function (key) {
          if (Cookies._cachedDocumentCookie !== Cookies._document.cookie) {
            Cookies._renewCache()
          }
          return Cookies._cache[Cookies._cacheKeyPrefix + key]
        };
        Cookies.set = function (key, value, options) {
          options = Cookies._getExtendedOptions(options);
          options.expires = Cookies._getExpiresDate(value === undefined ? -1 : options.expires);
          Cookies._document.cookie = Cookies._generateCookieString(key, value, options);
          return Cookies
        };
        Cookies.expire = function (key, options) {
          return Cookies.set(key, undefined, options)
        };
        Cookies._getExtendedOptions = function (options) {
          return {
            path: options && options.path || Cookies.defaults.path,
            domain: options && options.domain || Cookies.defaults.domain,
            expires: options && options.expires || Cookies.defaults.expires,
            secure: options && options.secure !== undefined ? options.secure : Cookies.defaults.secure
          }
        };
        Cookies._isValidDate = function (date) {
          return Object.prototype.toString.call(date) === '[object Date]' && !isNaN(date.getTime())
        };
        Cookies._getExpiresDate = function (expires, now) {
          now = now || new Date;
          if (typeof expires === 'number') {
            expires = expires === Infinity ? Cookies._maxExpireDate : new Date(now.getTime() + expires * 1000)
          } else if (typeof expires === 'string') {
            expires = new Date(expires)
          }
          if (expires && !Cookies._isValidDate(expires)) {
            throw new Error('`expires` parameter cannot be converted to a valid Date instance')
          }
          return expires
        };
        Cookies._generateCookieString = function (key, value, options) {
          key = key.replace(/[^#$&+\^`|]/g, encodeURIComponent);
          key = key.replace(/\(/g, '%28').replace(/\)/g, '%29');
          value = (value + '').replace(/[^!#$&-+\--:<-\[\]-~]/g, encodeURIComponent);
          options = options || {};
          var cookieString = key + '=' + value;
          cookieString += options.path ? ';path=' + options.path : '';
          cookieString += options.domain ? ';domain=' + options.domain : '';
          cookieString += options.expires ? ';expires=' + options.expires.toUTCString() : '';
          cookieString += options.secure ? ';secure' : '';
          return cookieString
        };
        Cookies._getCacheFromString = function (documentCookie) {
          var cookieCache = {};
          var cookiesArray = documentCookie ? documentCookie.split('; ') : [];
          for (var i = 0; i < cookiesArray.length; i++) {
            var cookieKvp = Cookies._getKeyValuePairFromCookieString(cookiesArray[i]);
            if (cookieCache[Cookies._cacheKeyPrefix + cookieKvp.key] === undefined) {
              cookieCache[Cookies._cacheKeyPrefix + cookieKvp.key] = cookieKvp.value
            }
          }
          return cookieCache
        };
        Cookies._getKeyValuePairFromCookieString = function (cookieString) {
          // "=" is a valid character in a cookie value according to RFC6265, so cannot `split('=')`
          var separatorIndex = cookieString.indexOf('=');
          // IE omits the "=" when the cookie value is an empty string
          separatorIndex = separatorIndex < 0 ? cookieString.length : separatorIndex;
          return {
            key: decodeURIComponent(cookieString.substr(0, separatorIndex)),
            value: decodeURIComponent(cookieString.substr(separatorIndex + 1))
          }
        };
        Cookies._renewCache = function () {
          Cookies._cache = Cookies._getCacheFromString(Cookies._document.cookie);
          Cookies._cachedDocumentCookie = Cookies._document.cookie
        };
        Cookies._areEnabled = function () {
          var testKey = 'cookies.js';
          var areEnabled = Cookies.set(testKey, 1).get(testKey) === '1';
          Cookies.expire(testKey);
          return areEnabled
        };
        Cookies.enabled = Cookies._areEnabled();
        return Cookies
      };
      var cookiesExport = typeof global.document === 'object' ? factory(global) : factory;
      // AMD support
      if (typeof define === 'function' && define.amd) {
        define(function () {
          return cookiesExport
        })  // CommonJS/Node.js support
      } else if (typeof exports === 'object') {
        // Support Node.js specific `module.exports` (which can be a function)
        if (typeof module === 'object' && typeof module.exports === 'object') {
          exports = module.exports = cookiesExport
        }
        // But always support CommonJS module 1.1.1 spec (`exports` cannot be a function)
        exports.Cookies = cookiesExport
      } else {
        global.Cookies = cookiesExport
      }
    }(typeof window === 'undefined' ? this : window))
  });
  // source: /Users/dtai/work/verus/crowdstart/node_modules/espy/node_modules/ua-parser-js/src/ua-parser.js
  require.define('espy/node_modules/ua-parser-js/src/ua-parser', function (module, exports, __dirname, __filename) {
    /**
 * UAParser.js v0.7.9
 * Lightweight JavaScript-based User-Agent string parser
 * https://github.com/faisalman/ua-parser-js
 *
 * Copyright © 2012-2015 Faisal Salman <fyzlman@gmail.com>
 * Dual licensed under GPLv2 & MIT
 */
    (function (window, undefined) {
      'use strict';
      //////////////
      // Constants
      /////////////
      var LIBVERSION = '0.7.9', EMPTY = '', UNKNOWN = '?', FUNC_TYPE = 'function', UNDEF_TYPE = 'undefined', OBJ_TYPE = 'object', STR_TYPE = 'string', MAJOR = 'major',
        // deprecated
        MODEL = 'model', NAME = 'name', TYPE = 'type', VENDOR = 'vendor', VERSION = 'version', ARCHITECTURE = 'architecture', CONSOLE = 'console', MOBILE = 'mobile', TABLET = 'tablet', SMARTTV = 'smarttv', WEARABLE = 'wearable', EMBEDDED = 'embedded';
      ///////////
      // Helper
      //////////
      var util = {
        extend: function (regexes, extensions) {
          for (var i in extensions) {
            if ('browser cpu device engine os'.indexOf(i) !== -1 && extensions[i].length % 2 === 0) {
              regexes[i] = extensions[i].concat(regexes[i])
            }
          }
          return regexes
        },
        has: function (str1, str2) {
          if (typeof str1 === 'string') {
            return str2.toLowerCase().indexOf(str1.toLowerCase()) !== -1
          } else {
            return false
          }
        },
        lowerize: function (str) {
          return str.toLowerCase()
        },
        major: function (version) {
          return typeof version === STR_TYPE ? version.split('.')[0] : undefined
        }
      };
      ///////////////
      // Map helper
      //////////////
      var mapper = {
        rgx: function () {
          var result, i = 0, j, k, p, q, matches, match, args = arguments;
          // loop through all regexes maps
          while (i < args.length && !matches) {
            var regex = args[i],
              // even sequence (0,2,4,..)
              props = args[i + 1];
            // odd sequence (1,3,5,..)
            // construct object barebones
            if (typeof result === UNDEF_TYPE) {
              result = {};
              for (p in props) {
                q = props[p];
                if (typeof q === OBJ_TYPE) {
                  result[q[0]] = undefined
                } else {
                  result[q] = undefined
                }
              }
            }
            // try matching uastring with regexes
            j = k = 0;
            while (j < regex.length && !matches) {
              matches = regex[j++].exec(this.getUA());
              if (!!matches) {
                for (p = 0; p < props.length; p++) {
                  match = matches[++k];
                  q = props[p];
                  // check if given property is actually array
                  if (typeof q === OBJ_TYPE && q.length > 0) {
                    if (q.length == 2) {
                      if (typeof q[1] == FUNC_TYPE) {
                        // assign modified match
                        result[q[0]] = q[1].call(this, match)
                      } else {
                        // assign given value, ignore regex match
                        result[q[0]] = q[1]
                      }
                    } else if (q.length == 3) {
                      // check whether function or regex
                      if (typeof q[1] === FUNC_TYPE && !(q[1].exec && q[1].test)) {
                        // call function (usually string mapper)
                        result[q[0]] = match ? q[1].call(this, match, q[2]) : undefined
                      } else {
                        // sanitize match using given regex
                        result[q[0]] = match ? match.replace(q[1], q[2]) : undefined
                      }
                    } else if (q.length == 4) {
                      result[q[0]] = match ? q[3].call(this, match.replace(q[1], q[2])) : undefined
                    }
                  } else {
                    result[q] = match ? match : undefined
                  }
                }
              }
            }
            i += 2
          }
          return result
        },
        str: function (str, map) {
          for (var i in map) {
            // check if array
            if (typeof map[i] === OBJ_TYPE && map[i].length > 0) {
              for (var j = 0; j < map[i].length; j++) {
                if (util.has(map[i][j], str)) {
                  return i === UNKNOWN ? undefined : i
                }
              }
            } else if (util.has(map[i], str)) {
              return i === UNKNOWN ? undefined : i
            }
          }
          return str
        }
      };
      ///////////////
      // String map
      //////////////
      var maps = {
        browser: {
          oldsafari: {
            version: {
              '1.0': '/8',
              '1.2': '/1',
              '1.3': '/3',
              '2.0': '/412',
              '2.0.2': '/416',
              '2.0.3': '/417',
              '2.0.4': '/419',
              '?': '/'
            }
          }
        },
        device: {
          amazon: {
            model: {
              'Fire Phone': [
                'SD',
                'KF'
              ]
            }
          },
          sprint: {
            model: { 'Evo Shift 4G': '7373KT' },
            vendor: {
              'HTC': 'APA',
              'Sprint': 'Sprint'
            }
          }
        },
        os: {
          windows: {
            version: {
              'ME': '4.90',
              'NT 3.11': 'NT3.51',
              'NT 4.0': 'NT4.0',
              '2000': 'NT 5.0',
              'XP': [
                'NT 5.1',
                'NT 5.2'
              ],
              'Vista': 'NT 6.0',
              '7': 'NT 6.1',
              '8': 'NT 6.2',
              '8.1': 'NT 6.3',
              '10': [
                'NT 6.4',
                'NT 10.0'
              ],
              'RT': 'ARM'
            }
          }
        }
      };
      //////////////
      // Regex map
      /////////////
      var regexes = {
        browser: [
          [
            // Presto based
            /(opera\smini)\/([\w\.-]+)/i,
            // Opera Mini
            /(opera\s[mobiletab]+).+version\/([\w\.-]+)/i,
            // Opera Mobi/Tablet
            /(opera).+version\/([\w\.]+)/i,
            // Opera > 9.80
            /(opera)[\/\s]+([\w\.]+)/i  // Opera < 9.80
          ],
          [
            NAME,
            VERSION
          ],
          [/\s(opr)\/([\w\.]+)/i  // Opera Webkit
],
          [
            [
              NAME,
              'Opera'
            ],
            VERSION
          ],
          [
            // Mixed
            /(kindle)\/([\w\.]+)/i,
            // Kindle
            /(lunascape|maxthon|netfront|jasmine|blazer)[\/\s]?([\w\.]+)*/i,
            // Lunascape/Maxthon/Netfront/Jasmine/Blazer
            // Trident based
            /(avant\s|iemobile|slim|baidu)(?:browser)?[\/\s]?([\w\.]*)/i,
            // Avant/IEMobile/SlimBrowser/Baidu
            /(?:ms|\()(ie)\s([\w\.]+)/i,
            // Internet Explorer
            // Webkit/KHTML based
            /(rekonq)\/([\w\.]+)*/i,
            // Rekonq
            /(chromium|flock|rockmelt|midori|epiphany|silk|skyfire|ovibrowser|bolt|iron|vivaldi|iridium)\/([\w\.-]+)/i  // Chromium/Flock/RockMelt/Midori/Epiphany/Silk/Skyfire/Bolt/Iron/Iridium
          ],
          [
            NAME,
            VERSION
          ],
          [/(trident).+rv[:\s]([\w\.]+).+like\sgecko/i  // IE11
],
          [
            [
              NAME,
              'IE'
            ],
            VERSION
          ],
          [/(edge)\/((\d+)?[\w\.]+)/i  // Microsoft Edge
],
          [
            NAME,
            VERSION
          ],
          [/(yabrowser)\/([\w\.]+)/i  // Yandex
],
          [
            [
              NAME,
              'Yandex'
            ],
            VERSION
          ],
          [/(comodo_dragon)\/([\w\.]+)/i  // Comodo Dragon
],
          [
            [
              NAME,
              /_/g,
              ' '
            ],
            VERSION
          ],
          [
            /(chrome|omniweb|arora|[tizenoka]{5}\s?browser)\/v?([\w\.]+)/i,
            // Chrome/OmniWeb/Arora/Tizen/Nokia
            /(uc\s?browser|qqbrowser)[\/\s]?([\w\.]+)/i  // UCBrowser/QQBrowser
          ],
          [
            NAME,
            VERSION
          ],
          [/(dolfin)\/([\w\.]+)/i  // Dolphin
],
          [
            [
              NAME,
              'Dolphin'
            ],
            VERSION
          ],
          [/((?:android.+)crmo|crios)\/([\w\.]+)/i  // Chrome for Android/iOS
],
          [
            [
              NAME,
              'Chrome'
            ],
            VERSION
          ],
          [/XiaoMi\/MiuiBrowser\/([\w\.]+)/i  // MIUI Browser
],
          [
            VERSION,
            [
              NAME,
              'MIUI Browser'
            ]
          ],
          [/android.+version\/([\w\.]+)\s+(?:mobile\s?safari|safari)/i  // Android Browser
],
          [
            VERSION,
            [
              NAME,
              'Android Browser'
            ]
          ],
          [/FBAV\/([\w\.]+);/i  // Facebook App for iOS
],
          [
            VERSION,
            [
              NAME,
              'Facebook'
            ]
          ],
          [/version\/([\w\.]+).+?mobile\/\w+\s(safari)/i  // Mobile Safari
],
          [
            VERSION,
            [
              NAME,
              'Mobile Safari'
            ]
          ],
          [/version\/([\w\.]+).+?(mobile\s?safari|safari)/i  // Safari & Safari Mobile
],
          [
            VERSION,
            NAME
          ],
          [/webkit.+?(mobile\s?safari|safari)(\/[\w\.]+)/i  // Safari < 3.0
],
          [
            NAME,
            [
              VERSION,
              mapper.str,
              maps.browser.oldsafari.version
            ]
          ],
          [
            /(konqueror)\/([\w\.]+)/i,
            // Konqueror
            /(webkit|khtml)\/([\w\.]+)/i
          ],
          [
            NAME,
            VERSION
          ],
          [// Gecko based
            /(navigator|netscape)\/([\w\.-]+)/i  // Netscape
],
          [
            [
              NAME,
              'Netscape'
            ],
            VERSION
          ],
          [/fxios\/([\w\.-]+)/i  // Firefox for iOS
],
          [
            VERSION,
            [
              NAME,
              'Firefox'
            ]
          ],
          [
            /(swiftfox)/i,
            // Swiftfox
            /(icedragon|iceweasel|camino|chimera|fennec|maemo\sbrowser|minimo|conkeror)[\/\s]?([\w\.\+]+)/i,
            // IceDragon/Iceweasel/Camino/Chimera/Fennec/Maemo/Minimo/Conkeror
            /(firefox|seamonkey|k-meleon|icecat|iceape|firebird|phoenix)\/([\w\.-]+)/i,
            // Firefox/SeaMonkey/K-Meleon/IceCat/IceApe/Firebird/Phoenix
            /(mozilla)\/([\w\.]+).+rv\:.+gecko\/\d+/i,
            // Mozilla
            // Other
            /(polaris|lynx|dillo|icab|doris|amaya|w3m|netsurf)[\/\s]?([\w\.]+)/i,
            // Polaris/Lynx/Dillo/iCab/Doris/Amaya/w3m/NetSurf
            /(links)\s\(([\w\.]+)/i,
            // Links
            /(gobrowser)\/?([\w\.]+)*/i,
            // GoBrowser
            /(ice\s?browser)\/v?([\w\._]+)/i,
            // ICE Browser
            /(mosaic)[\/\s]([\w\.]+)/i  // Mosaic
          ],
          [
            NAME,
            VERSION
          ]  /* /////////////////////
            // Media players BEGIN
            ////////////////////////

            , [

            /(apple(?:coremedia|))\/((\d+)[\w\._]+)/i,                          // Generic Apple CoreMedia
            /(coremedia) v((\d+)[\w\._]+)/i
            ], [NAME, VERSION], [

            /(aqualung|lyssna|bsplayer)\/((\d+)?[\w\.-]+)/i                     // Aqualung/Lyssna/BSPlayer
            ], [NAME, VERSION], [

            /(ares|ossproxy)\s((\d+)[\w\.-]+)/i                                 // Ares/OSSProxy
            ], [NAME, VERSION], [

            /(audacious|audimusicstream|amarok|bass|core|dalvik|gnomemplayer|music on console|nsplayer|psp-internetradioplayer|videos)\/((\d+)[\w\.-]+)/i,
                                                                                // Audacious/AudiMusicStream/Amarok/BASS/OpenCORE/Dalvik/GnomeMplayer/MoC
                                                                                // NSPlayer/PSP-InternetRadioPlayer/Videos
            /(clementine|music player daemon)\s((\d+)[\w\.-]+)/i,               // Clementine/MPD
            /(lg player|nexplayer)\s((\d+)[\d\.]+)/i,
            /player\/(nexplayer|lg player)\s((\d+)[\w\.-]+)/i                   // NexPlayer/LG Player
            ], [NAME, VERSION], [
            /(nexplayer)\s((\d+)[\w\.-]+)/i                                     // Nexplayer
            ], [NAME, VERSION], [

            /(flrp)\/((\d+)[\w\.-]+)/i                                          // Flip Player
            ], [[NAME, 'Flip Player'], VERSION], [

            /(fstream|nativehost|queryseekspider|ia-archiver|facebookexternalhit)/i
                                                                                // FStream/NativeHost/QuerySeekSpider/IA Archiver/facebookexternalhit
            ], [NAME], [

            /(gstreamer) souphttpsrc (?:\([^\)]+\)){0,1} libsoup\/((\d+)[\w\.-]+)/i
                                                                                // Gstreamer
            ], [NAME, VERSION], [

            /(htc streaming player)\s[\w_]+\s\/\s((\d+)[\d\.]+)/i,              // HTC Streaming Player
            /(java|python-urllib|python-requests|wget|libcurl)\/((\d+)[\w\.-_]+)/i,
                                                                                // Java/urllib/requests/wget/cURL
            /(lavf)((\d+)[\d\.]+)/i                                             // Lavf (FFMPEG)
            ], [NAME, VERSION], [

            /(htc_one_s)\/((\d+)[\d\.]+)/i                                      // HTC One S
            ], [[NAME, /_/g, ' '], VERSION], [

            /(mplayer)(?:\s|\/)(?:(?:sherpya-){0,1}svn)(?:-|\s)(r\d+(?:-\d+[\w\.-]+){0,1})/i
                                                                                // MPlayer SVN
            ], [NAME, VERSION], [

            /(mplayer)(?:\s|\/|[unkow-]+)((\d+)[\w\.-]+)/i                      // MPlayer
            ], [NAME, VERSION], [

            /(mplayer)/i,                                                       // MPlayer (no other info)
            /(yourmuze)/i,                                                      // YourMuze
            /(media player classic|nero showtime)/i                             // Media Player Classic/Nero ShowTime
            ], [NAME], [

            /(nero (?:home|scout))\/((\d+)[\w\.-]+)/i                           // Nero Home/Nero Scout
            ], [NAME, VERSION], [

            /(nokia\d+)\/((\d+)[\w\.-]+)/i                                      // Nokia
            ], [NAME, VERSION], [

            /\s(songbird)\/((\d+)[\w\.-]+)/i                                    // Songbird/Philips-Songbird
            ], [NAME, VERSION], [

            /(winamp)3 version ((\d+)[\w\.-]+)/i,                               // Winamp
            /(winamp)\s((\d+)[\w\.-]+)/i,
            /(winamp)mpeg\/((\d+)[\w\.-]+)/i
            ], [NAME, VERSION], [

            /(ocms-bot|tapinradio|tunein radio|unknown|winamp|inlight radio)/i  // OCMS-bot/tap in radio/tunein/unknown/winamp (no other info)
                                                                                // inlight radio
            ], [NAME], [

            /(quicktime|rma|radioapp|radioclientapplication|soundtap|totem|stagefright|streamium)\/((\d+)[\w\.-]+)/i
                                                                                // QuickTime/RealMedia/RadioApp/RadioClientApplication/
                                                                                // SoundTap/Totem/Stagefright/Streamium
            ], [NAME, VERSION], [

            /(smp)((\d+)[\d\.]+)/i                                              // SMP
            ], [NAME, VERSION], [

            /(vlc) media player - version ((\d+)[\w\.]+)/i,                     // VLC Videolan
            /(vlc)\/((\d+)[\w\.-]+)/i,
            /(xbmc|gvfs|xine|xmms|irapp)\/((\d+)[\w\.-]+)/i,                    // XBMC/gvfs/Xine/XMMS/irapp
            /(foobar2000)\/((\d+)[\d\.]+)/i,                                    // Foobar2000
            /(itunes)\/((\d+)[\d\.]+)/i                                         // iTunes
            ], [NAME, VERSION], [

            /(wmplayer)\/((\d+)[\w\.-]+)/i,                                     // Windows Media Player
            /(windows-media-player)\/((\d+)[\w\.-]+)/i
            ], [[NAME, /-/g, ' '], VERSION], [

            /windows\/((\d+)[\w\.-]+) upnp\/[\d\.]+ dlnadoc\/[\d\.]+ (home media server)/i
                                                                                // Windows Media Server
            ], [VERSION, [NAME, 'Windows']], [

            /(com\.riseupradioalarm)\/((\d+)[\d\.]*)/i                          // RiseUP Radio Alarm
            ], [NAME, VERSION], [

            /(rad.io)\s((\d+)[\d\.]+)/i,                                        // Rad.io
            /(radio.(?:de|at|fr))\s((\d+)[\d\.]+)/i
            ], [[NAME, 'rad.io'], VERSION]

            //////////////////////
            // Media players END
            ////////////////////*/
        ],
        cpu: [
          [/(?:(amd|x(?:(?:86|64)[_-])?|wow|win)64)[;\)]/i  // AMD64
],
          [[
              ARCHITECTURE,
              'amd64'
            ]],
          [/(ia32(?=;))/i  // IA32 (quicktime)
],
          [[
              ARCHITECTURE,
              util.lowerize
            ]],
          [/((?:i[346]|x)86)[;\)]/i  // IA32
],
          [[
              ARCHITECTURE,
              'ia32'
            ]],
          [// PocketPC mistakenly identified as PowerPC
            /windows\s(ce|mobile);\sppc;/i],
          [[
              ARCHITECTURE,
              'arm'
            ]],
          [/((?:ppc|powerpc)(?:64)?)(?:\smac|;|\))/i  // PowerPC
],
          [[
              ARCHITECTURE,
              /ower/,
              '',
              util.lowerize
            ]],
          [/(sun4\w)[;\)]/i  // SPARC
],
          [[
              ARCHITECTURE,
              'sparc'
            ]],
          [/((?:avr32|ia64(?=;))|68k(?=\))|arm(?:64|(?=v\d+;))|(?=atmel\s)avr|(?:irix|mips|sparc)(?:64)?(?=;)|pa-risc)/i  // IA64, 68K, ARM/64, AVR/32, IRIX/64, MIPS/64, SPARC/64, PA-RISC
],
          [[
              ARCHITECTURE,
              util.lowerize
            ]]
        ],
        device: [
          [/\((ipad|playbook);[\w\s\);-]+(rim|apple)/i  // iPad/PlayBook
],
          [
            MODEL,
            VENDOR,
            [
              TYPE,
              TABLET
            ]
          ],
          [/applecoremedia\/[\w\.]+ \((ipad)/  // iPad
],
          [
            MODEL,
            [
              VENDOR,
              'Apple'
            ],
            [
              TYPE,
              TABLET
            ]
          ],
          [/(apple\s{0,1}tv)/i  // Apple TV
],
          [
            [
              MODEL,
              'Apple TV'
            ],
            [
              VENDOR,
              'Apple'
            ]
          ],
          [
            /(archos)\s(gamepad2?)/i,
            // Archos
            /(hp).+(touchpad)/i,
            // HP TouchPad
            /(kindle)\/([\w\.]+)/i,
            // Kindle
            /\s(nook)[\w\s]+build\/(\w+)/i,
            // Nook
            /(dell)\s(strea[kpr\s\d]*[\dko])/i  // Dell Streak
          ],
          [
            VENDOR,
            MODEL,
            [
              TYPE,
              TABLET
            ]
          ],
          [/(kf[A-z]+)\sbuild\/[\w\.]+.*silk\//i  // Kindle Fire HD
],
          [
            MODEL,
            [
              VENDOR,
              'Amazon'
            ],
            [
              TYPE,
              TABLET
            ]
          ],
          [/(sd|kf)[0349hijorstuw]+\sbuild\/[\w\.]+.*silk\//i  // Fire Phone
],
          [
            [
              MODEL,
              mapper.str,
              maps.device.amazon.model
            ],
            [
              VENDOR,
              'Amazon'
            ],
            [
              TYPE,
              MOBILE
            ]
          ],
          [/\((ip[honed|\s\w*]+);.+(apple)/i  // iPod/iPhone
],
          [
            MODEL,
            VENDOR,
            [
              TYPE,
              MOBILE
            ]
          ],
          [/\((ip[honed|\s\w*]+);/i  // iPod/iPhone
],
          [
            MODEL,
            [
              VENDOR,
              'Apple'
            ],
            [
              TYPE,
              MOBILE
            ]
          ],
          [
            /(blackberry)[\s-]?(\w+)/i,
            // BlackBerry
            /(blackberry|benq|palm(?=\-)|sonyericsson|acer|asus|dell|huawei|meizu|motorola|polytron)[\s_-]?([\w-]+)*/i,
            // BenQ/Palm/Sony-Ericsson/Acer/Asus/Dell/Huawei/Meizu/Motorola/Polytron
            /(hp)\s([\w\s]+\w)/i,
            // HP iPAQ
            /(asus)-?(\w+)/i  // Asus
          ],
          [
            VENDOR,
            MODEL,
            [
              TYPE,
              MOBILE
            ]
          ],
          [/\(bb10;\s(\w+)/i  // BlackBerry 10
],
          [
            MODEL,
            [
              VENDOR,
              'BlackBerry'
            ],
            [
              TYPE,
              MOBILE
            ]
          ],
          [// Asus Tablets
            /android.+(transfo[prime\s]{4,10}\s\w+|eeepc|slider\s\w+|nexus 7)/i],
          [
            MODEL,
            [
              VENDOR,
              'Asus'
            ],
            [
              TYPE,
              TABLET
            ]
          ],
          [
            /(sony)\s(tablet\s[ps])\sbuild\//i,
            // Sony
            /(sony)?(?:sgp.+)\sbuild\//i
          ],
          [
            [
              VENDOR,
              'Sony'
            ],
            [
              MODEL,
              'Xperia Tablet'
            ],
            [
              TYPE,
              TABLET
            ]
          ],
          [/(?:sony)?(?:(?:(?:c|d)\d{4})|(?:so[-l].+))\sbuild\//i],
          [
            [
              VENDOR,
              'Sony'
            ],
            [
              MODEL,
              'Xperia Phone'
            ],
            [
              TYPE,
              MOBILE
            ]
          ],
          [
            /\s(ouya)\s/i,
            // Ouya
            /(nintendo)\s([wids3u]+)/i  // Nintendo
          ],
          [
            VENDOR,
            MODEL,
            [
              TYPE,
              CONSOLE
            ]
          ],
          [/android.+;\s(shield)\sbuild/i  // Nvidia
],
          [
            MODEL,
            [
              VENDOR,
              'Nvidia'
            ],
            [
              TYPE,
              CONSOLE
            ]
          ],
          [/(playstation\s[3portablevi]+)/i  // Playstation
],
          [
            MODEL,
            [
              VENDOR,
              'Sony'
            ],
            [
              TYPE,
              CONSOLE
            ]
          ],
          [/(sprint\s(\w+))/i  // Sprint Phones
],
          [
            [
              VENDOR,
              mapper.str,
              maps.device.sprint.vendor
            ],
            [
              MODEL,
              mapper.str,
              maps.device.sprint.model
            ],
            [
              TYPE,
              MOBILE
            ]
          ],
          [/(lenovo)\s?(S(?:5000|6000)+(?:[-][\w+]))/i  // Lenovo tablets
],
          [
            VENDOR,
            MODEL,
            [
              TYPE,
              TABLET
            ]
          ],
          [
            /(htc)[;_\s-]+([\w\s]+(?=\))|\w+)*/i,
            // HTC
            /(zte)-(\w+)*/i,
            // ZTE
            /(alcatel|geeksphone|huawei|lenovo|nexian|panasonic|(?=;\s)sony)[_\s-]?([\w-]+)*/i  // Alcatel/GeeksPhone/Huawei/Lenovo/Nexian/Panasonic/Sony
          ],
          [
            VENDOR,
            [
              MODEL,
              /_/g,
              ' '
            ],
            [
              TYPE,
              MOBILE
            ]
          ],
          [/(nexus\s9)/i  // HTC Nexus 9
],
          [
            MODEL,
            [
              VENDOR,
              'HTC'
            ],
            [
              TYPE,
              TABLET
            ]
          ],
          [/[\s\(;](xbox(?:\sone)?)[\s\);]/i  // Microsoft Xbox
],
          [
            MODEL,
            [
              VENDOR,
              'Microsoft'
            ],
            [
              TYPE,
              CONSOLE
            ]
          ],
          [/(kin\.[onetw]{3})/i  // Microsoft Kin
],
          [
            [
              MODEL,
              /\./g,
              ' '
            ],
            [
              VENDOR,
              'Microsoft'
            ],
            [
              TYPE,
              MOBILE
            ]
          ],
          [
            // Motorola
            /\s(milestone|droid(?:[2-4x]|\s(?:bionic|x2|pro|razr))?(:?\s4g)?)[\w\s]+build\//i,
            /mot[\s-]?(\w+)*/i,
            /(XT\d{3,4}) build\//i
          ],
          [
            MODEL,
            [
              VENDOR,
              'Motorola'
            ],
            [
              TYPE,
              MOBILE
            ]
          ],
          [/android.+\s(mz60\d|xoom[\s2]{0,2})\sbuild\//i],
          [
            MODEL,
            [
              VENDOR,
              'Motorola'
            ],
            [
              TYPE,
              TABLET
            ]
          ],
          [
            /android.+((sch-i[89]0\d|shw-m380s|gt-p\d{4}|gt-n8000|sgh-t8[56]9|nexus 10))/i,
            /((SM-T\w+))/i
          ],
          [
            [
              VENDOR,
              'Samsung'
            ],
            MODEL,
            [
              TYPE,
              TABLET
            ]
          ],
          [
            // Samsung
            /((s[cgp]h-\w+|gt-\w+|galaxy\snexus|sm-n900))/i,
            /(sam[sung]*)[\s-]*(\w+-?[\w-]*)*/i,
            /sec-((sgh\w+))/i
          ],
          [
            [
              VENDOR,
              'Samsung'
            ],
            MODEL,
            [
              TYPE,
              MOBILE
            ]
          ],
          [/(samsung);smarttv/i],
          [
            VENDOR,
            MODEL,
            [
              TYPE,
              SMARTTV
            ]
          ],
          [/\(dtv[\);].+(aquos)/i  // Sharp
],
          [
            MODEL,
            [
              VENDOR,
              'Sharp'
            ],
            [
              TYPE,
              SMARTTV
            ]
          ],
          [/sie-(\w+)*/i  // Siemens
],
          [
            MODEL,
            [
              VENDOR,
              'Siemens'
            ],
            [
              TYPE,
              MOBILE
            ]
          ],
          [
            /(maemo|nokia).*(n900|lumia\s\d+)/i,
            // Nokia
            /(nokia)[\s_-]?([\w-]+)*/i
          ],
          [
            [
              VENDOR,
              'Nokia'
            ],
            MODEL,
            [
              TYPE,
              MOBILE
            ]
          ],
          [/android\s3\.[\s\w;-]{10}(a\d{3})/i  // Acer
],
          [
            MODEL,
            [
              VENDOR,
              'Acer'
            ],
            [
              TYPE,
              TABLET
            ]
          ],
          [/android\s3\.[\s\w;-]{10}(lg?)-([06cv9]{3,4})/i  // LG Tablet
],
          [
            [
              VENDOR,
              'LG'
            ],
            MODEL,
            [
              TYPE,
              TABLET
            ]
          ],
          [/(lg) netcast\.tv/i  // LG SmartTV
],
          [
            VENDOR,
            MODEL,
            [
              TYPE,
              SMARTTV
            ]
          ],
          [
            /(nexus\s[45])/i,
            // LG
            /lg[e;\s\/-]+(\w+)*/i
          ],
          [
            MODEL,
            [
              VENDOR,
              'LG'
            ],
            [
              TYPE,
              MOBILE
            ]
          ],
          [/android.+(ideatab[a-z0-9\-\s]+)/i  // Lenovo
],
          [
            MODEL,
            [
              VENDOR,
              'Lenovo'
            ],
            [
              TYPE,
              TABLET
            ]
          ],
          [/linux;.+((jolla));/i  // Jolla
],
          [
            VENDOR,
            MODEL,
            [
              TYPE,
              MOBILE
            ]
          ],
          [/((pebble))app\/[\d\.]+\s/i  // Pebble
],
          [
            VENDOR,
            MODEL,
            [
              TYPE,
              WEARABLE
            ]
          ],
          [/android.+;\s(glass)\s\d/i  // Google Glass
],
          [
            MODEL,
            [
              VENDOR,
              'Google'
            ],
            [
              TYPE,
              WEARABLE
            ]
          ],
          [
            /android.+(\w+)\s+build\/hm\1/i,
            // Xiaomi Hongmi 'numeric' models
            /android.+(hm[\s\-_]*note?[\s_]*(?:\d\w)?)\s+build/i,
            // Xiaomi Hongmi
            /android.+(mi[\s\-_]*(?:one|one[\s_]plus)?[\s_]*(?:\d\w)?)\s+build/i  // Xiaomi Mi
          ],
          [
            [
              MODEL,
              /_/g,
              ' '
            ],
            [
              VENDOR,
              'Xiaomi'
            ],
            [
              TYPE,
              MOBILE
            ]
          ],
          [/(mobile|tablet);.+rv\:.+gecko\//i  // Unidentifiable
],
          [
            [
              TYPE,
              util.lowerize
            ],
            VENDOR,
            MODEL
          ]  /*//////////////////////////
            // TODO: move to string map
            ////////////////////////////

            /(C6603)/i                                                          // Sony Xperia Z C6603
            ], [[MODEL, 'Xperia Z C6603'], [VENDOR, 'Sony'], [TYPE, MOBILE]], [
            /(C6903)/i                                                          // Sony Xperia Z 1
            ], [[MODEL, 'Xperia Z 1'], [VENDOR, 'Sony'], [TYPE, MOBILE]], [

            /(SM-G900[F|H])/i                                                   // Samsung Galaxy S5
            ], [[MODEL, 'Galaxy S5'], [VENDOR, 'Samsung'], [TYPE, MOBILE]], [
            /(SM-G7102)/i                                                       // Samsung Galaxy Grand 2
            ], [[MODEL, 'Galaxy Grand 2'], [VENDOR, 'Samsung'], [TYPE, MOBILE]], [
            /(SM-G530H)/i                                                       // Samsung Galaxy Grand Prime
            ], [[MODEL, 'Galaxy Grand Prime'], [VENDOR, 'Samsung'], [TYPE, MOBILE]], [
            /(SM-G313HZ)/i                                                      // Samsung Galaxy V
            ], [[MODEL, 'Galaxy V'], [VENDOR, 'Samsung'], [TYPE, MOBILE]], [
            /(SM-T805)/i                                                        // Samsung Galaxy Tab S 10.5
            ], [[MODEL, 'Galaxy Tab S 10.5'], [VENDOR, 'Samsung'], [TYPE, TABLET]], [
            /(SM-G800F)/i                                                       // Samsung Galaxy S5 Mini
            ], [[MODEL, 'Galaxy S5 Mini'], [VENDOR, 'Samsung'], [TYPE, MOBILE]], [
            /(SM-T311)/i                                                        // Samsung Galaxy Tab 3 8.0
            ], [[MODEL, 'Galaxy Tab 3 8.0'], [VENDOR, 'Samsung'], [TYPE, TABLET]], [

            /(R1001)/i                                                          // Oppo R1001
            ], [MODEL, [VENDOR, 'OPPO'], [TYPE, MOBILE]], [
            /(X9006)/i                                                          // Oppo Find 7a
            ], [[MODEL, 'Find 7a'], [VENDOR, 'Oppo'], [TYPE, MOBILE]], [
            /(R2001)/i                                                          // Oppo YOYO R2001
            ], [[MODEL, 'Yoyo R2001'], [VENDOR, 'Oppo'], [TYPE, MOBILE]], [
            /(R815)/i                                                           // Oppo Clover R815
            ], [[MODEL, 'Clover R815'], [VENDOR, 'Oppo'], [TYPE, MOBILE]], [
             /(U707)/i                                                          // Oppo Find Way S
            ], [[MODEL, 'Find Way S'], [VENDOR, 'Oppo'], [TYPE, MOBILE]], [

            /(T3C)/i                                                            // Advan Vandroid T3C
            ], [MODEL, [VENDOR, 'Advan'], [TYPE, TABLET]], [
            /(ADVAN T1J\+)/i                                                    // Advan Vandroid T1J+
            ], [[MODEL, 'Vandroid T1J+'], [VENDOR, 'Advan'], [TYPE, TABLET]], [
            /(ADVAN S4A)/i                                                      // Advan Vandroid S4A
            ], [[MODEL, 'Vandroid S4A'], [VENDOR, 'Advan'], [TYPE, MOBILE]], [

            /(V972M)/i                                                          // ZTE V972M
            ], [MODEL, [VENDOR, 'ZTE'], [TYPE, MOBILE]], [

            /(i-mobile)\s(IQ\s[\d\.]+)/i                                        // i-mobile IQ
            ], [VENDOR, MODEL, [TYPE, MOBILE]], [
            /(IQ6.3)/i                                                          // i-mobile IQ IQ 6.3
            ], [[MODEL, 'IQ 6.3'], [VENDOR, 'i-mobile'], [TYPE, MOBILE]], [
            /(i-mobile)\s(i-style\s[\d\.]+)/i                                   // i-mobile i-STYLE
            ], [VENDOR, MODEL, [TYPE, MOBILE]], [
            /(i-STYLE2.1)/i                                                     // i-mobile i-STYLE 2.1
            ], [[MODEL, 'i-STYLE 2.1'], [VENDOR, 'i-mobile'], [TYPE, MOBILE]], [
            
            /(mobiistar touch LAI 512)/i                                        // mobiistar touch LAI 512
            ], [[MODEL, 'Touch LAI 512'], [VENDOR, 'mobiistar'], [TYPE, MOBILE]], [

            /////////////
            // END TODO
            ///////////*/
        ],
        engine: [
          [/windows.+\sedge\/([\w\.]+)/i  // EdgeHTML
],
          [
            VERSION,
            [
              NAME,
              'EdgeHTML'
            ]
          ],
          [
            /(presto)\/([\w\.]+)/i,
            // Presto
            /(webkit|trident|netfront|netsurf|amaya|lynx|w3m)\/([\w\.]+)/i,
            // WebKit/Trident/NetFront/NetSurf/Amaya/Lynx/w3m
            /(khtml|tasman|links)[\/\s]\(?([\w\.]+)/i,
            // KHTML/Tasman/Links
            /(icab)[\/\s]([23]\.[\d\.]+)/i  // iCab
          ],
          [
            NAME,
            VERSION
          ],
          [/rv\:([\w\.]+).*(gecko)/i  // Gecko
],
          [
            VERSION,
            NAME
          ]
        ],
        os: [
          [// Windows based
            /microsoft\s(windows)\s(vista|xp)/i  // Windows (iTunes)
],
          [
            NAME,
            VERSION
          ],
          [
            /(windows)\snt\s6\.2;\s(arm)/i,
            // Windows RT
            /(windows\sphone(?:\sos)*|windows\smobile|windows)[\s\/]?([ntce\d\.\s]+\w)/i
          ],
          [
            NAME,
            [
              VERSION,
              mapper.str,
              maps.os.windows.version
            ]
          ],
          [/(win(?=3|9|n)|win\s9x\s)([nt\d\.]+)/i],
          [
            [
              NAME,
              'Windows'
            ],
            [
              VERSION,
              mapper.str,
              maps.os.windows.version
            ]
          ],
          [// Mobile/Embedded OS
            /\((bb)(10);/i  // BlackBerry 10
],
          [
            [
              NAME,
              'BlackBerry'
            ],
            VERSION
          ],
          [
            /(blackberry)\w*\/?([\w\.]+)*/i,
            // Blackberry
            /(tizen)[\/\s]([\w\.]+)/i,
            // Tizen
            /(android|webos|palm\sos|qnx|bada|rim\stablet\sos|meego|contiki)[\/\s-]?([\w\.]+)*/i,
            // Android/WebOS/Palm/QNX/Bada/RIM/MeeGo/Contiki
            /linux;.+(sailfish);/i  // Sailfish OS
          ],
          [
            NAME,
            VERSION
          ],
          [/(symbian\s?os|symbos|s60(?=;))[\/\s-]?([\w\.]+)*/i  // Symbian
],
          [
            [
              NAME,
              'Symbian'
            ],
            VERSION
          ],
          [/\((series40);/i  // Series 40
],
          [NAME],
          [/mozilla.+\(mobile;.+gecko.+firefox/i  // Firefox OS
],
          [
            [
              NAME,
              'Firefox OS'
            ],
            VERSION
          ],
          [
            // Console
            /(nintendo|playstation)\s([wids3portablevu]+)/i,
            // Nintendo/Playstation
            // GNU/Linux based
            /(mint)[\/\s\(]?(\w+)*/i,
            // Mint
            /(mageia|vectorlinux)[;\s]/i,
            // Mageia/VectorLinux
            /(joli|[kxln]?ubuntu|debian|[open]*suse|gentoo|arch|slackware|fedora|mandriva|centos|pclinuxos|redhat|zenwalk|linpus)[\/\s-]?([\w\.-]+)*/i,
            // Joli/Ubuntu/Debian/SUSE/Gentoo/Arch/Slackware
            // Fedora/Mandriva/CentOS/PCLinuxOS/RedHat/Zenwalk/Linpus
            /(hurd|linux)\s?([\w\.]+)*/i,
            // Hurd/Linux
            /(gnu)\s?([\w\.]+)*/i  // GNU
          ],
          [
            NAME,
            VERSION
          ],
          [/(cros)\s[\w]+\s([\w\.]+\w)/i  // Chromium OS
],
          [
            [
              NAME,
              'Chromium OS'
            ],
            VERSION
          ],
          [// Solaris
            /(sunos)\s?([\w\.]+\d)*/i  // Solaris
],
          [
            [
              NAME,
              'Solaris'
            ],
            VERSION
          ],
          [// BSD based
            /\s([frentopc-]{0,4}bsd|dragonfly)\s?([\w\.]+)*/i  // FreeBSD/NetBSD/OpenBSD/PC-BSD/DragonFly
],
          [
            NAME,
            VERSION
          ],
          [/(ip[honead]+)(?:.*os\s*([\w]+)*\slike\smac|;\sopera)/i  // iOS
],
          [
            [
              NAME,
              'iOS'
            ],
            [
              VERSION,
              /_/g,
              '.'
            ]
          ],
          [
            /(mac\sos\sx)\s?([\w\s\.]+\w)*/i,
            /(macintosh|mac(?=_powerpc)\s)/i  // Mac OS
          ],
          [
            [
              NAME,
              'Mac OS'
            ],
            [
              VERSION,
              /_/g,
              '.'
            ]
          ],
          [
            // Other
            /((?:open)?solaris)[\/\s-]?([\w\.]+)*/i,
            // Solaris
            /(haiku)\s(\w+)/i,
            // Haiku
            /(aix)\s((\d)(?=\.|\)|\s)[\w\.]*)*/i,
            // AIX
            /(plan\s9|minix|beos|os\/2|amigaos|morphos|risc\sos|openvms)/i,
            // Plan9/Minix/BeOS/OS2/AmigaOS/MorphOS/RISCOS/OpenVMS
            /(unix)\s?([\w\.]+)*/i  // UNIX
          ],
          [
            NAME,
            VERSION
          ]
        ]
      };
      /////////////////
      // Constructor
      ////////////////
      var UAParser = function (uastring, extensions) {
        if (!(this instanceof UAParser)) {
          return new UAParser(uastring, extensions).getResult()
        }
        var ua = uastring || (window && window.navigator && window.navigator.userAgent ? window.navigator.userAgent : EMPTY);
        var rgxmap = extensions ? util.extend(regexes, extensions) : regexes;
        this.getBrowser = function () {
          var browser = mapper.rgx.apply(this, rgxmap.browser);
          browser.major = util.major(browser.version);
          return browser
        };
        this.getCPU = function () {
          return mapper.rgx.apply(this, rgxmap.cpu)
        };
        this.getDevice = function () {
          return mapper.rgx.apply(this, rgxmap.device)
        };
        this.getEngine = function () {
          return mapper.rgx.apply(this, rgxmap.engine)
        };
        this.getOS = function () {
          return mapper.rgx.apply(this, rgxmap.os)
        };
        this.getResult = function () {
          return {
            ua: this.getUA(),
            browser: this.getBrowser(),
            engine: this.getEngine(),
            os: this.getOS(),
            device: this.getDevice(),
            cpu: this.getCPU()
          }
        };
        this.getUA = function () {
          return ua
        };
        this.setUA = function (uastring) {
          ua = uastring;
          return this
        };
        this.setUA(ua);
        return this
      };
      UAParser.VERSION = LIBVERSION;
      UAParser.BROWSER = {
        NAME: NAME,
        MAJOR: MAJOR,
        // deprecated
        VERSION: VERSION
      };
      UAParser.CPU = { ARCHITECTURE: ARCHITECTURE };
      UAParser.DEVICE = {
        MODEL: MODEL,
        VENDOR: VENDOR,
        TYPE: TYPE,
        CONSOLE: CONSOLE,
        MOBILE: MOBILE,
        SMARTTV: SMARTTV,
        TABLET: TABLET,
        WEARABLE: WEARABLE,
        EMBEDDED: EMBEDDED
      };
      UAParser.ENGINE = {
        NAME: NAME,
        VERSION: VERSION
      };
      UAParser.OS = {
        NAME: NAME,
        VERSION: VERSION
      };
      ///////////
      // Export
      //////////
      // check js environment
      if (typeof exports !== UNDEF_TYPE) {
        // nodejs env
        if (typeof module !== UNDEF_TYPE && module.exports) {
          exports = module.exports = UAParser
        }
        exports.UAParser = UAParser
      } else {
        // requirejs env (optional)
        if (typeof define === FUNC_TYPE && define.amd) {
          define(function () {
            return UAParser
          })
        } else {
          // browser env
          window.UAParser = UAParser
        }
      }
      // jQuery/Zepto specific (optional)
      // Note: 
      //   In AMD env the global scope should be kept clean, but jQuery is an exception.
      //   jQuery always exports to global scope, unless jQuery.noConflict(true) is used,
      //   and we should catch that.
      var $ = window.jQuery || window.Zepto;
      if (typeof $ !== UNDEF_TYPE) {
        var parser = new UAParser;
        $.ua = parser.getResult();
        $.ua.get = function () {
          return parser.getUA()
        };
        $.ua.set = function (uastring) {
          parser.setUA(uastring);
          var result = parser.getResult();
          for (var prop in result) {
            $.ua[prop] = result[prop]
          }
        }
      }
    }(typeof window === 'object' ? window : this))
  });
  // source: /Users/dtai/work/verus/crowdstart/node_modules/espy/node_modules/query-string/index.js
  require.define('espy/node_modules/query-string', function (module, exports, __dirname, __filename) {
    'use strict';
    var strictUriEncode = require('espy/node_modules/query-string/node_modules/strict-uri-encode');
    exports.extract = function (str) {
      return str.split('?')[1] || ''
    };
    exports.parse = function (str) {
      if (typeof str !== 'string') {
        return {}
      }
      str = str.trim().replace(/^(\?|#|&)/, '');
      if (!str) {
        return {}
      }
      return str.split('&').reduce(function (ret, param) {
        var parts = param.replace(/\+/g, ' ').split('=');
        var key = parts[0];
        var val = parts[1];
        key = decodeURIComponent(key);
        // missing `=` should be `null`:
        // http://w3.org/TR/2012/WD-url-20120524/#collect-url-parameters
        val = val === undefined ? null : decodeURIComponent(val);
        if (!ret.hasOwnProperty(key)) {
          ret[key] = val
        } else if (Array.isArray(ret[key])) {
          ret[key].push(val)
        } else {
          ret[key] = [
            ret[key],
            val
          ]
        }
        return ret
      }, {})
    };
    exports.stringify = function (obj) {
      return obj ? Object.keys(obj).sort().map(function (key) {
        var val = obj[key];
        if (Array.isArray(val)) {
          return val.sort().map(function (val2) {
            return strictUriEncode(key) + '=' + strictUriEncode(val2)
          }).join('&')
        }
        return strictUriEncode(key) + '=' + strictUriEncode(val)
      }).join('&') : ''
    }
  });
  // source: /Users/dtai/work/verus/crowdstart/node_modules/espy/node_modules/query-string/node_modules/strict-uri-encode/index.js
  require.define('espy/node_modules/query-string/node_modules/strict-uri-encode', function (module, exports, __dirname, __filename) {
    'use strict';
    module.exports = function (str) {
      return encodeURIComponent(str).replace(/[!'()*]/g, function (c) {
        return '%' + c.charCodeAt(0).toString(16)
      })
    }
  });
  // source: /Users/dtai/work/verus/crowdstart/node_modules/espy/node_modules/node-uuid/uuid.js
  require.define('espy/node_modules/node-uuid/uuid', function (module, exports, __dirname, __filename) {
    //     uuid.js
    //
    //     Copyright (c) 2010-2012 Robert Kieffer
    //     MIT License - http://opensource.org/licenses/mit-license.php
    (function () {
      var _global = this;
      // Unique ID creation requires a high quality random # generator.  We feature
      // detect to determine the best RNG source, normalizing to a function that
      // returns 128-bits of randomness, since that's what's usually required
      var _rng;
      // Node.js crypto-based RNG - http://nodejs.org/docs/v0.6.2/api/crypto.html
      //
      // Moderately fast, high quality
      if (typeof _global.require == 'function') {
        try {
          var _rb = _global.require('crypto').randomBytes;
          _rng = _rb && function () {
            return _rb(16)
          }
        } catch (e) {
        }
      }
      if (!_rng && _global.crypto && crypto.getRandomValues) {
        // WHATWG crypto-based RNG - http://wiki.whatwg.org/wiki/Crypto
        //
        // Moderately fast, high quality
        var _rnds8 = new Uint8Array(16);
        _rng = function whatwgRNG() {
          crypto.getRandomValues(_rnds8);
          return _rnds8
        }
      }
      if (!_rng) {
        // Math.random()-based (RNG)
        //
        // If all else fails, use Math.random().  It's fast, but is of unspecified
        // quality.
        var _rnds = new Array(16);
        _rng = function () {
          for (var i = 0, r; i < 16; i++) {
            if ((i & 3) === 0)
              r = Math.random() * 4294967296;
            _rnds[i] = r >>> ((i & 3) << 3) & 255
          }
          return _rnds
        }
      }
      // Buffer class to use
      var BufferClass = typeof _global.Buffer == 'function' ? _global.Buffer : Array;
      // Maps for number <-> hex string conversion
      var _byteToHex = [];
      var _hexToByte = {};
      for (var i = 0; i < 256; i++) {
        _byteToHex[i] = (i + 256).toString(16).substr(1);
        _hexToByte[_byteToHex[i]] = i
      }
      // **`parse()` - Parse a UUID into it's component bytes**
      function parse(s, buf, offset) {
        var i = buf && offset || 0, ii = 0;
        buf = buf || [];
        s.toLowerCase().replace(/[0-9a-f]{2}/g, function (oct) {
          if (ii < 16) {
            // Don't overflow!
            buf[i + ii++] = _hexToByte[oct]
          }
        });
        // Zero out remaining bytes if string was short
        while (ii < 16) {
          buf[i + ii++] = 0
        }
        return buf
      }
      // **`unparse()` - Convert UUID byte array (ala parse()) into a string**
      function unparse(buf, offset) {
        var i = offset || 0, bth = _byteToHex;
        return bth[buf[i++]] + bth[buf[i++]] + bth[buf[i++]] + bth[buf[i++]] + '-' + bth[buf[i++]] + bth[buf[i++]] + '-' + bth[buf[i++]] + bth[buf[i++]] + '-' + bth[buf[i++]] + bth[buf[i++]] + '-' + bth[buf[i++]] + bth[buf[i++]] + bth[buf[i++]] + bth[buf[i++]] + bth[buf[i++]] + bth[buf[i++]]
      }
      // **`v1()` - Generate time-based UUID**
      //
      // Inspired by https://github.com/LiosK/UUID.js
      // and http://docs.python.org/library/uuid.html
      // random #'s we need to init node and clockseq
      var _seedBytes = _rng();
      // Per 4.5, create and 48-bit node id, (47 random bits + multicast bit = 1)
      var _nodeId = [
        _seedBytes[0] | 1,
        _seedBytes[1],
        _seedBytes[2],
        _seedBytes[3],
        _seedBytes[4],
        _seedBytes[5]
      ];
      // Per 4.2.2, randomize (14 bit) clockseq
      var _clockseq = (_seedBytes[6] << 8 | _seedBytes[7]) & 16383;
      // Previous uuid creation time
      var _lastMSecs = 0, _lastNSecs = 0;
      // See https://github.com/broofa/node-uuid for API details
      function v1(options, buf, offset) {
        var i = buf && offset || 0;
        var b = buf || [];
        options = options || {};
        var clockseq = options.clockseq != null ? options.clockseq : _clockseq;
        // UUID timestamps are 100 nano-second units since the Gregorian epoch,
        // (1582-10-15 00:00).  JSNumbers aren't precise enough for this, so
        // time is handled internally as 'msecs' (integer milliseconds) and 'nsecs'
        // (100-nanoseconds offset from msecs) since unix epoch, 1970-01-01 00:00.
        var msecs = options.msecs != null ? options.msecs : new Date().getTime();
        // Per 4.2.1.2, use count of uuid's generated during the current clock
        // cycle to simulate higher resolution clock
        var nsecs = options.nsecs != null ? options.nsecs : _lastNSecs + 1;
        // Time since last uuid creation (in msecs)
        var dt = msecs - _lastMSecs + (nsecs - _lastNSecs) / 10000;
        // Per 4.2.1.2, Bump clockseq on clock regression
        if (dt < 0 && options.clockseq == null) {
          clockseq = clockseq + 1 & 16383
        }
        // Reset nsecs if clock regresses (new clockseq) or we've moved onto a new
        // time interval
        if ((dt < 0 || msecs > _lastMSecs) && options.nsecs == null) {
          nsecs = 0
        }
        // Per 4.2.1.2 Throw error if too many uuids are requested
        if (nsecs >= 10000) {
          throw new Error("uuid.v1(): Can't create more than 10M uuids/sec")
        }
        _lastMSecs = msecs;
        _lastNSecs = nsecs;
        _clockseq = clockseq;
        // Per 4.1.4 - Convert from unix epoch to Gregorian epoch
        msecs += 12219292800000;
        // `time_low`
        var tl = ((msecs & 268435455) * 10000 + nsecs) % 4294967296;
        b[i++] = tl >>> 24 & 255;
        b[i++] = tl >>> 16 & 255;
        b[i++] = tl >>> 8 & 255;
        b[i++] = tl & 255;
        // `time_mid`
        var tmh = msecs / 4294967296 * 10000 & 268435455;
        b[i++] = tmh >>> 8 & 255;
        b[i++] = tmh & 255;
        // `time_high_and_version`
        b[i++] = tmh >>> 24 & 15 | 16;
        // include version
        b[i++] = tmh >>> 16 & 255;
        // `clock_seq_hi_and_reserved` (Per 4.2.2 - include variant)
        b[i++] = clockseq >>> 8 | 128;
        // `clock_seq_low`
        b[i++] = clockseq & 255;
        // `node`
        var node = options.node || _nodeId;
        for (var n = 0; n < 6; n++) {
          b[i + n] = node[n]
        }
        return buf ? buf : unparse(b)
      }
      // **`v4()` - Generate random UUID**
      // See https://github.com/broofa/node-uuid for API details
      function v4(options, buf, offset) {
        // Deprecated - 'format' argument, as supported in v1.2
        var i = buf && offset || 0;
        if (typeof options == 'string') {
          buf = options == 'binary' ? new BufferClass(16) : null;
          options = null
        }
        options = options || {};
        var rnds = options.random || (options.rng || _rng)();
        // Per 4.4, set bits for version and `clock_seq_hi_and_reserved`
        rnds[6] = rnds[6] & 15 | 64;
        rnds[8] = rnds[8] & 63 | 128;
        // Copy bytes to buffer, if provided
        if (buf) {
          for (var ii = 0; ii < 16; ii++) {
            buf[i + ii] = rnds[ii]
          }
        }
        return buf || unparse(rnds)
      }
      // Export public API
      var uuid = v4;
      uuid.v1 = v1;
      uuid.v4 = v4;
      uuid.parse = parse;
      uuid.unparse = unparse;
      uuid.BufferClass = BufferClass;
      if (typeof module != 'undefined' && module.exports) {
        // Publish as node.js module
        module.exports = uuid
      } else if (typeof define === 'function' && define.amd) {
        // Publish as AMD module
        define(function () {
          return uuid
        })
      } else {
        // Publish as global (in browsers)
        var _previousRoot = _global.uuid;
        // **`noConflict()` - (browser only) to reset global 'uuid' var**
        uuid.noConflict = function () {
          _global.uuid = _previousRoot;
          return uuid
        };
        _global.uuid = uuid
      }
    }.call(this))
  });
  // source: /Users/dtai/work/verus/crowdstart/node_modules/cuckoo-js/src/index.coffee
  require.define('cuckoo-js/src', function (module, exports, __dirname, __filename) {
    var exports;
    exports = {
      Egg: function () {
        return console.log('Egg called with ', arguments)
      }
    };
    (function () {
      var addEventListener, proto, removeEventListener;
      if (typeof EventTarget !== 'undefined' && EventTarget !== null) {
        proto = EventTarget.prototype
      } else if (typeof Node !== 'undefined' && Node !== null) {
        proto = Node.prototype
      } else {
        if (typeof console !== 'undefined' && console !== null) {
          if (typeof console.log === 'function') {
            console.log('EventTarget and Node are missing')
          }
        }
        return
      }
      if (proto) {
        if (proto.addEventListener == null) {
          if (typeof console !== 'undefined' && console !== null) {
            if (typeof console.log === 'function') {
              console.log('addEventListener is missing')
            }
          }
          return
        }
        if (proto.removeEventListener == null) {
          if (typeof console !== 'undefined' && console !== null) {
            if (typeof console.log === 'function') {
              console.log('removeEventListener is missing')
            }
          }
          return
        }
        addEventListener = proto.addEventListener;
        proto.addEventListener = function (type, listener, useCapture) {
          var l, nest;
          l = listener;
          nest = function (event) {
            var e;
            try {
              if (!event.__reported) {
                exports.Egg.apply(this, arguments);
                Object.defineProperty(event, '__reported', {
                  value: true,
                  writable: true
                })
              }
            } catch (_error) {
              e = _error
            }
            l.apply(this, arguments);
            try {
              return Object.defineProperty(l, '__nest', {
                value: nest,
                writable: true
              })
            } catch (_error) {
            }
          };
          return addEventListener.call(this, type, nest, useCapture)
        };
        removeEventListener = proto.removeEventListener;
        proto.removeEventListener = function (type, listener, useCapture) {
          var nest;
          try {
            nest = listener.__nest
          } catch (_error) {
          }
          if (nest == null) {
            nest = listener
          }
          return removeEventListener(type, nest, useCapture)
        }
      }
      exports.Target = function (types) {
        var event, events, i, len, results;
        events = types.split(' ');
        results = [];
        for (i = 0, len = events.length; i < len; i++) {
          event = events[i];
          results.push(window.addEventListener(event, function () {
            return exports.Egg.apply(exports.Egg, arguments)
          }))
        }
        return results
      };
      if (typeof window !== 'undefined' && window !== null) {
        return window.cuckoo = exports
      }
    }());
    module.exports = exports
  });
  // source: /Users/dtai/work/verus/crowdstart/assets/js/analytics/native.coffee
  require.define('./native', function (module, exports, __dirname, __filename) {
    (function () {
      var Cuckoo, Espy, debounced;
      Espy = require('espy/src');
      Cuckoo = require('cuckoo-js/src');
      Espy.url = '%%%%%url%%%%%';
      Cuckoo.Target('click touch submit');
      debounced = {};
      return Cuckoo.Egg = function (event) {
        var clas, eventName, id, name, type;
        type = event.type;
        eventName = type;
        if (type === 'click' || type === 'touch' || type === 'submit') {
          eventName += '_' + event.target.tagName;
          id = event.target.getAttribute('id');
          if (id) {
            eventName += '#' + id
          } else {
            name = event.target.getAttribute('name');
            if (name) {
              eventName += '[name=' + name + ']'
            } else {
              clas = event.target.getAttribute('class');
              if (clas) {
                eventName += '.' + clas.replace(/ /g, '.')
              }
            }
          }
          if (debounced[eventName] == null) {
            Espy(eventName)
          }
        } else if (type === 'scroll') {
          if (debounced[eventName] == null) {
            Espy(eventName, {
              scrollX: window.scrollX,
              scrollY: window.scrollY
            })
          }
        }
        return debounced[eventName] = setTimeout(function () {
          return debounced[eventName] = void 0
        }, 100)
      }
    }())
  });
  require('./native')
}.call(this, this))