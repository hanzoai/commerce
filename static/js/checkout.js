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
  // source: /Users/dtai/work/verus/crowdstart/node_modules/card/lib/js/card.js
  require.define('card/lib/js/card', function (module, exports, __dirname, __filename) {
    (function e(t, n, r) {
      function s(o, u) {
        if (!n[o]) {
          if (!t[o]) {
            var a = typeof require == 'function' && require;
            if (!u && a)
              return a(o, !0);
            if (i)
              return i(o, !0);
            var f = new Error("Cannot find module '" + o + "'");
            throw f.code = 'MODULE_NOT_FOUND', f
          }
          var l = n[o] = { exports: {} };
          t[o][0].call(l.exports, function (e) {
            var n = t[o][1][e];
            return s(n ? n : e)
          }, l, l.exports, e, t, n, r)
        }
        return n[o].exports
      }
      var i = typeof require == 'function' && require;
      for (var o = 0; o < r.length; o++)
        s(r[o]);
      return s
    }({
      1: [
        function (require, module, exports) {
          module.exports = require('./lib/extend')
        },
        { './lib/extend': 2 }
      ],
      2: [
        function (require, module, exports) {
          /*!
 * node.extend
 * Copyright 2011, John Resig
 * Dual licensed under the MIT or GPL Version 2 licenses.
 * http://jquery.org/license
 *
 * @fileoverview
 * Port of jQuery.extend that actually works on node.js
 */
          var is = require('is');
          function extend() {
            var target = arguments[0] || {};
            var i = 1;
            var length = arguments.length;
            var deep = false;
            var options, name, src, copy, copy_is_array, clone;
            // Handle a deep copy situation
            if (typeof target === 'boolean') {
              deep = target;
              target = arguments[1] || {};
              // skip the boolean and the target
              i = 2
            }
            // Handle case when target is a string or something (possible in deep copy)
            if (typeof target !== 'object' && !is.fn(target)) {
              target = {}
            }
            for (; i < length; i++) {
              // Only deal with non-null/undefined values
              options = arguments[i];
              if (options != null) {
                if (typeof options === 'string') {
                  options = options.split('')
                }
                // Extend the base object
                for (name in options) {
                  src = target[name];
                  copy = options[name];
                  // Prevent never-ending loop
                  if (target === copy) {
                    continue
                  }
                  // Recurse if we're merging plain objects or arrays
                  if (deep && copy && (is.hash(copy) || (copy_is_array = is.array(copy)))) {
                    if (copy_is_array) {
                      copy_is_array = false;
                      clone = src && is.array(src) ? src : []
                    } else {
                      clone = src && is.hash(src) ? src : {}
                    }
                    // Never move original objects, clone them
                    target[name] = extend(deep, clone, copy)  // Don't bring in undefined values
                  } else if (typeof copy !== 'undefined') {
                    target[name] = copy
                  }
                }
              }
            }
            // Return the modified object
            return target
          }
          ;
          /**
 * @public
 */
          extend.version = '1.1.3';
          /**
 * Exports module.
 */
          module.exports = extend
        },
        { 'is': 3 }
      ],
      3: [
        function (require, module, exports) {
          /**!
 * is
 * the definitive JavaScript type testing library
 *
 * @copyright 2013-2014 Enrico Marino / Jordan Harband
 * @license MIT
 */
          var objProto = Object.prototype;
          var owns = objProto.hasOwnProperty;
          var toStr = objProto.toString;
          var symbolValueOf;
          if (typeof Symbol === 'function') {
            symbolValueOf = Symbol.prototype.valueOf
          }
          var isActualNaN = function (value) {
            return value !== value
          };
          var NON_HOST_TYPES = {
            boolean: 1,
            number: 1,
            string: 1,
            undefined: 1
          };
          var base64Regex = /^([A-Za-z0-9+/]{4})*([A-Za-z0-9+/]{4}|[A-Za-z0-9+/]{3}=|[A-Za-z0-9+/]{2}==)$/;
          var hexRegex = /^[A-Fa-f0-9]+$/;
          /**
 * Expose `is`
 */
          var is = module.exports = {};
          /**
 * Test general.
 */
          /**
 * is.type
 * Test if `value` is a type of `type`.
 *
 * @param {Mixed} value value to test
 * @param {String} type type
 * @return {Boolean} true if `value` is a type of `type`, false otherwise
 * @api public
 */
          is.a = is.type = function (value, type) {
            return typeof value === type
          };
          /**
 * is.defined
 * Test if `value` is defined.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if 'value' is defined, false otherwise
 * @api public
 */
          is.defined = function (value) {
            return typeof value !== 'undefined'
          };
          /**
 * is.empty
 * Test if `value` is empty.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is empty, false otherwise
 * @api public
 */
          is.empty = function (value) {
            var type = toStr.call(value);
            var key;
            if ('[object Array]' === type || '[object Arguments]' === type || '[object String]' === type) {
              return value.length === 0
            }
            if ('[object Object]' === type) {
              for (key in value) {
                if (owns.call(value, key)) {
                  return false
                }
              }
              return true
            }
            return !value
          };
          /**
 * is.equal
 * Test if `value` is equal to `other`.
 *
 * @param {Mixed} value value to test
 * @param {Mixed} other value to compare with
 * @return {Boolean} true if `value` is equal to `other`, false otherwise
 */
          is.equal = function (value, other) {
            var strictlyEqual = value === other;
            if (strictlyEqual) {
              return true
            }
            var type = toStr.call(value);
            var key;
            if (type !== toStr.call(other)) {
              return false
            }
            if ('[object Object]' === type) {
              for (key in value) {
                if (!is.equal(value[key], other[key]) || !(key in other)) {
                  return false
                }
              }
              for (key in other) {
                if (!is.equal(value[key], other[key]) || !(key in value)) {
                  return false
                }
              }
              return true
            }
            if ('[object Array]' === type) {
              key = value.length;
              if (key !== other.length) {
                return false
              }
              while (--key) {
                if (!is.equal(value[key], other[key])) {
                  return false
                }
              }
              return true
            }
            if ('[object Function]' === type) {
              return value.prototype === other.prototype
            }
            if ('[object Date]' === type) {
              return value.getTime() === other.getTime()
            }
            return strictlyEqual
          };
          /**
 * is.hosted
 * Test if `value` is hosted by `host`.
 *
 * @param {Mixed} value to test
 * @param {Mixed} host host to test with
 * @return {Boolean} true if `value` is hosted by `host`, false otherwise
 * @api public
 */
          is.hosted = function (value, host) {
            var type = typeof host[value];
            return type === 'object' ? !!host[value] : !NON_HOST_TYPES[type]
          };
          /**
 * is.instance
 * Test if `value` is an instance of `constructor`.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is an instance of `constructor`
 * @api public
 */
          is.instance = is['instanceof'] = function (value, constructor) {
            return value instanceof constructor
          };
          /**
 * is.nil / is.null
 * Test if `value` is null.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is null, false otherwise
 * @api public
 */
          is.nil = is['null'] = function (value) {
            return value === null
          };
          /**
 * is.undef / is.undefined
 * Test if `value` is undefined.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is undefined, false otherwise
 * @api public
 */
          is.undef = is.undefined = function (value) {
            return typeof value === 'undefined'
          };
          /**
 * Test arguments.
 */
          /**
 * is.args
 * Test if `value` is an arguments object.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is an arguments object, false otherwise
 * @api public
 */
          is.args = is.arguments = function (value) {
            var isStandardArguments = '[object Arguments]' === toStr.call(value);
            var isOldArguments = !is.array(value) && is.arraylike(value) && is.object(value) && is.fn(value.callee);
            return isStandardArguments || isOldArguments
          };
          /**
 * Test array.
 */
          /**
 * is.array
 * Test if 'value' is an array.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is an array, false otherwise
 * @api public
 */
          is.array = function (value) {
            return '[object Array]' === toStr.call(value)
          };
          /**
 * is.arguments.empty
 * Test if `value` is an empty arguments object.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is an empty arguments object, false otherwise
 * @api public
 */
          is.args.empty = function (value) {
            return is.args(value) && value.length === 0
          };
          /**
 * is.array.empty
 * Test if `value` is an empty array.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is an empty array, false otherwise
 * @api public
 */
          is.array.empty = function (value) {
            return is.array(value) && value.length === 0
          };
          /**
 * is.arraylike
 * Test if `value` is an arraylike object.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is an arguments object, false otherwise
 * @api public
 */
          is.arraylike = function (value) {
            return !!value && !is.boolean(value) && owns.call(value, 'length') && isFinite(value.length) && is.number(value.length) && value.length >= 0
          };
          /**
 * Test boolean.
 */
          /**
 * is.boolean
 * Test if `value` is a boolean.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is a boolean, false otherwise
 * @api public
 */
          is.boolean = function (value) {
            return '[object Boolean]' === toStr.call(value)
          };
          /**
 * is.false
 * Test if `value` is false.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is false, false otherwise
 * @api public
 */
          is['false'] = function (value) {
            return is.boolean(value) && Boolean(Number(value)) === false
          };
          /**
 * is.true
 * Test if `value` is true.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is true, false otherwise
 * @api public
 */
          is['true'] = function (value) {
            return is.boolean(value) && Boolean(Number(value)) === true
          };
          /**
 * Test date.
 */
          /**
 * is.date
 * Test if `value` is a date.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is a date, false otherwise
 * @api public
 */
          is.date = function (value) {
            return '[object Date]' === toStr.call(value)
          };
          /**
 * Test element.
 */
          /**
 * is.element
 * Test if `value` is an html element.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is an HTML Element, false otherwise
 * @api public
 */
          is.element = function (value) {
            return value !== undefined && typeof HTMLElement !== 'undefined' && value instanceof HTMLElement && value.nodeType === 1
          };
          /**
 * Test error.
 */
          /**
 * is.error
 * Test if `value` is an error object.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is an error object, false otherwise
 * @api public
 */
          is.error = function (value) {
            return '[object Error]' === toStr.call(value)
          };
          /**
 * Test function.
 */
          /**
 * is.fn / is.function (deprecated)
 * Test if `value` is a function.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is a function, false otherwise
 * @api public
 */
          is.fn = is['function'] = function (value) {
            var isAlert = typeof window !== 'undefined' && value === window.alert;
            return isAlert || '[object Function]' === toStr.call(value)
          };
          /**
 * Test number.
 */
          /**
 * is.number
 * Test if `value` is a number.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is a number, false otherwise
 * @api public
 */
          is.number = function (value) {
            return '[object Number]' === toStr.call(value)
          };
          /**
 * is.infinite
 * Test if `value` is positive or negative infinity.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is positive or negative Infinity, false otherwise
 * @api public
 */
          is.infinite = function (value) {
            return value === Infinity || value === -Infinity
          };
          /**
 * is.decimal
 * Test if `value` is a decimal number.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is a decimal number, false otherwise
 * @api public
 */
          is.decimal = function (value) {
            return is.number(value) && !isActualNaN(value) && !is.infinite(value) && value % 1 !== 0
          };
          /**
 * is.divisibleBy
 * Test if `value` is divisible by `n`.
 *
 * @param {Number} value value to test
 * @param {Number} n dividend
 * @return {Boolean} true if `value` is divisible by `n`, false otherwise
 * @api public
 */
          is.divisibleBy = function (value, n) {
            var isDividendInfinite = is.infinite(value);
            var isDivisorInfinite = is.infinite(n);
            var isNonZeroNumber = is.number(value) && !isActualNaN(value) && is.number(n) && !isActualNaN(n) && n !== 0;
            return isDividendInfinite || isDivisorInfinite || isNonZeroNumber && value % n === 0
          };
          /**
 * is.int
 * Test if `value` is an integer.
 *
 * @param value to test
 * @return {Boolean} true if `value` is an integer, false otherwise
 * @api public
 */
          is.int = function (value) {
            return is.number(value) && !isActualNaN(value) && value % 1 === 0
          };
          /**
 * is.maximum
 * Test if `value` is greater than 'others' values.
 *
 * @param {Number} value value to test
 * @param {Array} others values to compare with
 * @return {Boolean} true if `value` is greater than `others` values
 * @api public
 */
          is.maximum = function (value, others) {
            if (isActualNaN(value)) {
              throw new TypeError('NaN is not a valid value')
            } else if (!is.arraylike(others)) {
              throw new TypeError('second argument must be array-like')
            }
            var len = others.length;
            while (--len >= 0) {
              if (value < others[len]) {
                return false
              }
            }
            return true
          };
          /**
 * is.minimum
 * Test if `value` is less than `others` values.
 *
 * @param {Number} value value to test
 * @param {Array} others values to compare with
 * @return {Boolean} true if `value` is less than `others` values
 * @api public
 */
          is.minimum = function (value, others) {
            if (isActualNaN(value)) {
              throw new TypeError('NaN is not a valid value')
            } else if (!is.arraylike(others)) {
              throw new TypeError('second argument must be array-like')
            }
            var len = others.length;
            while (--len >= 0) {
              if (value > others[len]) {
                return false
              }
            }
            return true
          };
          /**
 * is.nan
 * Test if `value` is not a number.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is not a number, false otherwise
 * @api public
 */
          is.nan = function (value) {
            return !is.number(value) || value !== value
          };
          /**
 * is.even
 * Test if `value` is an even number.
 *
 * @param {Number} value value to test
 * @return {Boolean} true if `value` is an even number, false otherwise
 * @api public
 */
          is.even = function (value) {
            return is.infinite(value) || is.number(value) && value === value && value % 2 === 0
          };
          /**
 * is.odd
 * Test if `value` is an odd number.
 *
 * @param {Number} value value to test
 * @return {Boolean} true if `value` is an odd number, false otherwise
 * @api public
 */
          is.odd = function (value) {
            return is.infinite(value) || is.number(value) && value === value && value % 2 !== 0
          };
          /**
 * is.ge
 * Test if `value` is greater than or equal to `other`.
 *
 * @param {Number} value value to test
 * @param {Number} other value to compare with
 * @return {Boolean}
 * @api public
 */
          is.ge = function (value, other) {
            if (isActualNaN(value) || isActualNaN(other)) {
              throw new TypeError('NaN is not a valid value')
            }
            return !is.infinite(value) && !is.infinite(other) && value >= other
          };
          /**
 * is.gt
 * Test if `value` is greater than `other`.
 *
 * @param {Number} value value to test
 * @param {Number} other value to compare with
 * @return {Boolean}
 * @api public
 */
          is.gt = function (value, other) {
            if (isActualNaN(value) || isActualNaN(other)) {
              throw new TypeError('NaN is not a valid value')
            }
            return !is.infinite(value) && !is.infinite(other) && value > other
          };
          /**
 * is.le
 * Test if `value` is less than or equal to `other`.
 *
 * @param {Number} value value to test
 * @param {Number} other value to compare with
 * @return {Boolean} if 'value' is less than or equal to 'other'
 * @api public
 */
          is.le = function (value, other) {
            if (isActualNaN(value) || isActualNaN(other)) {
              throw new TypeError('NaN is not a valid value')
            }
            return !is.infinite(value) && !is.infinite(other) && value <= other
          };
          /**
 * is.lt
 * Test if `value` is less than `other`.
 *
 * @param {Number} value value to test
 * @param {Number} other value to compare with
 * @return {Boolean} if `value` is less than `other`
 * @api public
 */
          is.lt = function (value, other) {
            if (isActualNaN(value) || isActualNaN(other)) {
              throw new TypeError('NaN is not a valid value')
            }
            return !is.infinite(value) && !is.infinite(other) && value < other
          };
          /**
 * is.within
 * Test if `value` is within `start` and `finish`.
 *
 * @param {Number} value value to test
 * @param {Number} start lower bound
 * @param {Number} finish upper bound
 * @return {Boolean} true if 'value' is is within 'start' and 'finish'
 * @api public
 */
          is.within = function (value, start, finish) {
            if (isActualNaN(value) || isActualNaN(start) || isActualNaN(finish)) {
              throw new TypeError('NaN is not a valid value')
            } else if (!is.number(value) || !is.number(start) || !is.number(finish)) {
              throw new TypeError('all arguments must be numbers')
            }
            var isAnyInfinite = is.infinite(value) || is.infinite(start) || is.infinite(finish);
            return isAnyInfinite || value >= start && value <= finish
          };
          /**
 * Test object.
 */
          /**
 * is.object
 * Test if `value` is an object.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is an object, false otherwise
 * @api public
 */
          is.object = function (value) {
            return '[object Object]' === toStr.call(value)
          };
          /**
 * is.hash
 * Test if `value` is a hash - a plain object literal.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is a hash, false otherwise
 * @api public
 */
          is.hash = function (value) {
            return is.object(value) && value.constructor === Object && !value.nodeType && !value.setInterval
          };
          /**
 * Test regexp.
 */
          /**
 * is.regexp
 * Test if `value` is a regular expression.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is a regexp, false otherwise
 * @api public
 */
          is.regexp = function (value) {
            return '[object RegExp]' === toStr.call(value)
          };
          /**
 * Test string.
 */
          /**
 * is.string
 * Test if `value` is a string.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if 'value' is a string, false otherwise
 * @api public
 */
          is.string = function (value) {
            return '[object String]' === toStr.call(value)
          };
          /**
 * Test base64 string.
 */
          /**
 * is.base64
 * Test if `value` is a valid base64 encoded string.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if 'value' is a base64 encoded string, false otherwise
 * @api public
 */
          is.base64 = function (value) {
            return is.string(value) && (!value.length || base64Regex.test(value))
          };
          /**
 * Test base64 string.
 */
          /**
 * is.hex
 * Test if `value` is a valid hex encoded string.
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if 'value' is a hex encoded string, false otherwise
 * @api public
 */
          is.hex = function (value) {
            return is.string(value) && (!value.length || hexRegex.test(value))
          };
          /**
 * is.symbol
 * Test if `value` is an ES6 Symbol
 *
 * @param {Mixed} value value to test
 * @return {Boolean} true if `value` is a Symbol, false otherise
 * @api public
 */
          is.symbol = function (value) {
            return typeof Symbol === 'function' && toStr.call(value) === '[object Symbol]' && typeof symbolValueOf.call(value) === 'symbol'
          }
        },
        {}
      ],
      4: [
        function (require, module, exports) {
          (function (global) {
            !function (e) {
              if ('object' == typeof exports && 'undefined' != typeof module)
                module.exports = e();
              else if ('function' == typeof define && define.amd)
                define([], e);
              else {
                var f;
                'undefined' != typeof window ? f = window : 'undefined' != typeof global ? f = global : 'undefined' != typeof self && (f = self), (f.qj || (f.qj = {})).js = e()
              }
            }(function () {
              var define, module, exports;
              return function e(t, n, r) {
                function s(o, u) {
                  if (!n[o]) {
                    if (!t[o]) {
                      var a = typeof require == 'function' && require;
                      if (!u && a)
                        return a(o, !0);
                      if (i)
                        return i(o, !0);
                      throw new Error("Cannot find module '" + o + "'")
                    }
                    var f = n[o] = { exports: {} };
                    t[o][0].call(f.exports, function (e) {
                      var n = t[o][1][e];
                      return s(n ? n : e)
                    }, f, f.exports, e, t, n, r)
                  }
                  return n[o].exports
                }
                var i = typeof require == 'function' && require;
                for (var o = 0; o < r.length; o++)
                  s(r[o]);
                return s
              }({
                1: [
                  function (_dereq_, module, exports) {
                    var QJ, rreturn, rtrim;
                    QJ = function (selector) {
                      if (QJ.isDOMElement(selector)) {
                        return selector
                      }
                      return document.querySelectorAll(selector)
                    };
                    QJ.isDOMElement = function (el) {
                      return el && el.nodeName != null
                    };
                    rtrim = /^[\s\uFEFF\xA0]+|[\s\uFEFF\xA0]+$/g;
                    QJ.trim = function (text) {
                      if (text === null) {
                        return ''
                      } else {
                        return (text + '').replace(rtrim, '')
                      }
                    };
                    rreturn = /\r/g;
                    QJ.val = function (el, val) {
                      var ret;
                      if (arguments.length > 1) {
                        return el.value = val
                      } else {
                        ret = el.value;
                        if (typeof ret === 'string') {
                          return ret.replace(rreturn, '')
                        } else {
                          if (ret === null) {
                            return ''
                          } else {
                            return ret
                          }
                        }
                      }
                    };
                    QJ.preventDefault = function (eventObject) {
                      if (typeof eventObject.preventDefault === 'function') {
                        eventObject.preventDefault();
                        return
                      }
                      eventObject.returnValue = false;
                      return false
                    };
                    QJ.normalizeEvent = function (e) {
                      var original;
                      original = e;
                      e = {
                        which: original.which != null ? original.which : void 0,
                        target: original.target || original.srcElement,
                        preventDefault: function () {
                          return QJ.preventDefault(original)
                        },
                        originalEvent: original,
                        data: original.data || original.detail
                      };
                      if (e.which == null) {
                        e.which = original.charCode != null ? original.charCode : original.keyCode
                      }
                      return e
                    };
                    QJ.on = function (element, eventName, callback) {
                      var el, multEventName, originalCallback, _i, _j, _len, _len1, _ref;
                      if (element.length) {
                        for (_i = 0, _len = element.length; _i < _len; _i++) {
                          el = element[_i];
                          QJ.on(el, eventName, callback)
                        }
                        return
                      }
                      if (eventName.match(' ')) {
                        _ref = eventName.split(' ');
                        for (_j = 0, _len1 = _ref.length; _j < _len1; _j++) {
                          multEventName = _ref[_j];
                          QJ.on(element, multEventName, callback)
                        }
                        return
                      }
                      originalCallback = callback;
                      callback = function (e) {
                        e = QJ.normalizeEvent(e);
                        return originalCallback(e)
                      };
                      if (element.addEventListener) {
                        return element.addEventListener(eventName, callback, false)
                      }
                      if (element.attachEvent) {
                        eventName = 'on' + eventName;
                        return element.attachEvent(eventName, callback)
                      }
                      element['on' + eventName] = callback
                    };
                    QJ.addClass = function (el, className) {
                      var e;
                      if (el.length) {
                        return function () {
                          var _i, _len, _results;
                          _results = [];
                          for (_i = 0, _len = el.length; _i < _len; _i++) {
                            e = el[_i];
                            _results.push(QJ.addClass(e, className))
                          }
                          return _results
                        }()
                      }
                      if (el.classList) {
                        return el.classList.add(className)
                      } else {
                        return el.className += ' ' + className
                      }
                    };
                    QJ.hasClass = function (el, className) {
                      var e, hasClass, _i, _len;
                      if (el.length) {
                        hasClass = true;
                        for (_i = 0, _len = el.length; _i < _len; _i++) {
                          e = el[_i];
                          hasClass = hasClass && QJ.hasClass(e, className)
                        }
                        return hasClass
                      }
                      if (el.classList) {
                        return el.classList.contains(className)
                      } else {
                        return new RegExp('(^| )' + className + '( |$)', 'gi').test(el.className)
                      }
                    };
                    QJ.removeClass = function (el, className) {
                      var cls, e, _i, _len, _ref, _results;
                      if (el.length) {
                        return function () {
                          var _i, _len, _results;
                          _results = [];
                          for (_i = 0, _len = el.length; _i < _len; _i++) {
                            e = el[_i];
                            _results.push(QJ.removeClass(e, className))
                          }
                          return _results
                        }()
                      }
                      if (el.classList) {
                        _ref = className.split(' ');
                        _results = [];
                        for (_i = 0, _len = _ref.length; _i < _len; _i++) {
                          cls = _ref[_i];
                          _results.push(el.classList.remove(cls))
                        }
                        return _results
                      } else {
                        return el.className = el.className.replace(new RegExp('(^|\\b)' + className.split(' ').join('|') + '(\\b|$)', 'gi'), ' ')
                      }
                    };
                    QJ.toggleClass = function (el, className, bool) {
                      var e;
                      if (el.length) {
                        return function () {
                          var _i, _len, _results;
                          _results = [];
                          for (_i = 0, _len = el.length; _i < _len; _i++) {
                            e = el[_i];
                            _results.push(QJ.toggleClass(e, className, bool))
                          }
                          return _results
                        }()
                      }
                      if (bool) {
                        if (!QJ.hasClass(el, className)) {
                          return QJ.addClass(el, className)
                        }
                      } else {
                        return QJ.removeClass(el, className)
                      }
                    };
                    QJ.append = function (el, toAppend) {
                      var e;
                      if (el.length) {
                        return function () {
                          var _i, _len, _results;
                          _results = [];
                          for (_i = 0, _len = el.length; _i < _len; _i++) {
                            e = el[_i];
                            _results.push(QJ.append(e, toAppend))
                          }
                          return _results
                        }()
                      }
                      return el.insertAdjacentHTML('beforeend', toAppend)
                    };
                    QJ.find = function (el, selector) {
                      if (el instanceof NodeList || el instanceof Array) {
                        el = el[0]
                      }
                      return el.querySelectorAll(selector)
                    };
                    QJ.trigger = function (el, name, data) {
                      var e, ev;
                      try {
                        ev = new CustomEvent(name, { detail: data })
                      } catch (_error) {
                        e = _error;
                        ev = document.createEvent('CustomEvent');
                        if (ev.initCustomEvent) {
                          ev.initCustomEvent(name, true, true, data)
                        } else {
                          ev.initEvent(name, true, true, data)
                        }
                      }
                      return el.dispatchEvent(ev)
                    };
                    module.exports = QJ
                  },
                  {}
                ]
              }, {}, [1])(1)
            })
          }.call(this, typeof global !== 'undefined' ? global : typeof self !== 'undefined' ? self : typeof window !== 'undefined' ? window : {}))
        },
        {}
      ],
      5: [
        function (require, module, exports) {
          module.exports = require('cssify')
        },
        { 'cssify': 6 }
      ],
      6: [
        function (require, module, exports) {
          module.exports = function (css, customDocument) {
            var doc = customDocument || document;
            if (doc.createStyleSheet) {
              var sheet = doc.createStyleSheet();
              sheet.cssText = css;
              return sheet.ownerNode
            } else {
              var head = doc.getElementsByTagName('head')[0], style = doc.createElement('style');
              style.type = 'text/css';
              if (style.styleSheet) {
                style.styleSheet.cssText = css
              } else {
                style.appendChild(doc.createTextNode(css))
              }
              head.appendChild(style);
              return style
            }
          };
          module.exports.byUrl = function (url) {
            if (document.createStyleSheet) {
              return document.createStyleSheet(url).ownerNode
            } else {
              var head = document.getElementsByTagName('head')[0], link = document.createElement('link');
              link.rel = 'stylesheet';
              link.href = url;
              head.appendChild(link);
              return link
            }
          }
        },
        {}
      ],
      7: [
        function (require, module, exports) {
          (function (global) {
            var Card, QJ, extend, payment;
            require('../scss/card.scss');
            QJ = require('qj');
            payment = require('./payment/src/payment.coffee');
            extend = require('node.extend');
            Card = function () {
              var bindVal;
              Card.prototype.cardTemplate = '' + '<div class="jp-card-container">' + '<div class="jp-card">' + '<div class="jp-card-front">' + '<div class="jp-card-logo jp-card-visa">visa</div>' + '<div class="jp-card-logo jp-card-mastercard">MasterCard</div>' + '<div class="jp-card-logo jp-card-maestro">Maestro</div>' + '<div class="jp-card-logo jp-card-amex"></div>' + '<div class="jp-card-logo jp-card-discover">discover</div>' + '<div class="jp-card-logo jp-card-dankort"><div class="dk"><div class="d"></div><div class="k"></div></div></div>' + '<div class="jp-card-lower">' + '<div class="jp-card-shiny"></div>' + '<div class="jp-card-cvc jp-card-display">{{cvc}}</div>' + '<div class="jp-card-number jp-card-display">{{number}}</div>' + '<div class="jp-card-name jp-card-display">{{name}}</div>' + '<div class="jp-card-expiry jp-card-display" data-before="{{monthYear}}" data-after="{{validDate}}">{{expiry}}</div>' + '</div>' + '</div>' + '<div class="jp-card-back">' + '<div class="jp-card-bar"></div>' + '<div class="jp-card-cvc jp-card-display">{{cvc}}</div>' + '<div class="jp-card-shiny"></div>' + '</div>' + '</div>' + '</div>';
              Card.prototype.template = function (tpl, data) {
                return tpl.replace(/\{\{(.*?)\}\}/g, function (match, key, str) {
                  return data[key]
                })
              };
              Card.prototype.cardTypes = [
                'jp-card-amex',
                'jp-card-dankort',
                'jp-card-dinersclub',
                'jp-card-discover',
                'jp-card-jcb',
                'jp-card-laser',
                'jp-card-maestro',
                'jp-card-mastercard',
                'jp-card-unionpay',
                'jp-card-visa',
                'jp-card-visaelectron'
              ];
              Card.prototype.defaults = {
                formatting: true,
                formSelectors: {
                  numberInput: 'input[name="number"]',
                  expiryInput: 'input[name="expiry"]',
                  cvcInput: 'input[name="cvc"]',
                  nameInput: 'input[name="name"]'
                },
                cardSelectors: {
                  cardContainer: '.jp-card-container',
                  card: '.jp-card',
                  numberDisplay: '.jp-card-number',
                  expiryDisplay: '.jp-card-expiry',
                  cvcDisplay: '.jp-card-cvc',
                  nameDisplay: '.jp-card-name'
                },
                messages: {
                  validDate: 'valid\nthru',
                  monthYear: 'month/year'
                },
                placeholders: {
                  number: '&bull;&bull;&bull;&bull; &bull;&bull;&bull;&bull; &bull;&bull;&bull;&bull; &bull;&bull;&bull;&bull;',
                  cvc: '&bull;&bull;&bull;',
                  expiry: '&bull;&bull;/&bull;&bull;',
                  name: 'Full Name'
                },
                classes: {
                  valid: 'jp-card-valid',
                  invalid: 'jp-card-invalid'
                },
                debug: false
              };
              function Card(opts) {
                this.options = extend(true, this.defaults, opts);
                if (!this.options.form) {
                  console.log('Please provide a form');
                  return
                }
                this.$el = QJ(this.options.form);
                if (!this.options.container) {
                  console.log('Please provide a container');
                  return
                }
                this.$container = QJ(this.options.container);
                this.render();
                this.attachHandlers();
                this.handleInitialPlaceholders()
              }
              Card.prototype.render = function () {
                var $cardContainer, baseWidth, name, obj, selector, ua, _ref, _ref1;
                QJ.append(this.$container, this.template(this.cardTemplate, extend({}, this.options.messages, this.options.placeholders)));
                _ref = this.options.cardSelectors;
                for (name in _ref) {
                  selector = _ref[name];
                  this['$' + name] = QJ.find(this.$container, selector)
                }
                _ref1 = this.options.formSelectors;
                for (name in _ref1) {
                  selector = _ref1[name];
                  selector = this.options[name] ? this.options[name] : selector;
                  obj = QJ.find(this.$el, selector);
                  if (!obj.length && this.options.debug) {
                    console.error("Card can't find a " + name + ' in your form.')
                  }
                  this['$' + name] = obj
                }
                if (this.options.formatting) {
                  Payment.formatCardNumber(this.$numberInput);
                  Payment.formatCardCVC(this.$cvcInput);
                  if (this.$expiryInput.length === 1) {
                    Payment.formatCardExpiry(this.$expiryInput)
                  }
                }
                if (this.options.width) {
                  $cardContainer = QJ(this.options.cardSelectors.cardContainer)[0];
                  baseWidth = parseInt($cardContainer.clientWidth);
                  $cardContainer.style.transform = 'scale(' + this.options.width / baseWidth + ')'
                }
                if (typeof navigator !== 'undefined' && navigator !== null ? navigator.userAgent : void 0) {
                  ua = navigator.userAgent.toLowerCase();
                  if (ua.indexOf('safari') !== -1 && ua.indexOf('chrome') === -1) {
                    QJ.addClass(this.$card, 'jp-card-safari')
                  }
                }
                if (/MSIE 10\./i.test(navigator.userAgent)) {
                  QJ.addClass(this.$card, 'jp-card-ie-10')
                }
                if (/rv:11.0/i.test(navigator.userAgent)) {
                  return QJ.addClass(this.$card, 'jp-card-ie-11')
                }
              };
              Card.prototype.attachHandlers = function () {
                var expiryFilters;
                bindVal(this.$numberInput, this.$numberDisplay, {
                  fill: false,
                  filters: this.validToggler('cardNumber')
                });
                QJ.on(this.$numberInput, 'payment.cardType', this.handle('setCardType'));
                expiryFilters = [function (val) {
                    return val.replace(/(\s+)/g, '')
                  }];
                if (this.$expiryInput.length === 1) {
                  expiryFilters.push(this.validToggler('cardExpiry'))
                }
                bindVal(this.$expiryInput, this.$expiryDisplay, {
                  join: function (text) {
                    if (text[0].length === 2 || text[1]) {
                      return '/'
                    } else {
                      return ''
                    }
                  },
                  filters: expiryFilters
                });
                bindVal(this.$cvcInput, this.$cvcDisplay, { filters: this.validToggler('cardCVC') });
                QJ.on(this.$cvcInput, 'focus', this.handle('flipCard'));
                QJ.on(this.$cvcInput, 'blur', this.handle('unflipCard'));
                return bindVal(this.$nameInput, this.$nameDisplay, {
                  fill: false,
                  filters: this.validToggler('cardHolderName'),
                  join: ' '
                })
              };
              Card.prototype.handleInitialPlaceholders = function () {
                var el, name, selector, _ref, _results;
                _ref = this.options.formSelectors;
                _results = [];
                for (name in _ref) {
                  selector = _ref[name];
                  el = this['$' + name];
                  if (QJ.val(el)) {
                    QJ.trigger(el, 'paste');
                    _results.push(setTimeout(function () {
                      return QJ.trigger(el, 'keyup')
                    }))
                  } else {
                    _results.push(void 0)
                  }
                }
                return _results
              };
              Card.prototype.handle = function (fn) {
                return function (_this) {
                  return function (e) {
                    var args;
                    args = Array.prototype.slice.call(arguments);
                    args.unshift(e.target);
                    return _this.handlers[fn].apply(_this, args)
                  }
                }(this)
              };
              Card.prototype.validToggler = function (validatorName) {
                var isValid;
                if (validatorName === 'cardExpiry') {
                  isValid = function (val) {
                    var objVal;
                    objVal = Payment.fns.cardExpiryVal(val);
                    return Payment.fns.validateCardExpiry(objVal.month, objVal.year)
                  }
                } else if (validatorName === 'cardCVC') {
                  isValid = function (_this) {
                    return function (val) {
                      return Payment.fns.validateCardCVC(val, _this.cardType)
                    }
                  }(this)
                } else if (validatorName === 'cardNumber') {
                  isValid = function (val) {
                    return Payment.fns.validateCardNumber(val)
                  }
                } else if (validatorName === 'cardHolderName') {
                  isValid = function (val) {
                    return val !== ''
                  }
                }
                return function (_this) {
                  return function (val, $in, $out) {
                    var result;
                    result = isValid(val);
                    _this.toggleValidClass($in, result);
                    _this.toggleValidClass($out, result);
                    return val
                  }
                }(this)
              };
              Card.prototype.toggleValidClass = function (el, test) {
                QJ.toggleClass(el, this.options.classes.valid, test);
                return QJ.toggleClass(el, this.options.classes.invalid, !test)
              };
              Card.prototype.handlers = {
                setCardType: function ($el, e) {
                  var cardType;
                  cardType = e.data;
                  if (!QJ.hasClass(this.$card, cardType)) {
                    QJ.removeClass(this.$card, 'jp-card-unknown');
                    QJ.removeClass(this.$card, this.cardTypes.join(' '));
                    QJ.addClass(this.$card, 'jp-card-' + cardType);
                    QJ.toggleClass(this.$card, 'jp-card-identified', cardType !== 'unknown');
                    return this.cardType = cardType
                  }
                },
                flipCard: function () {
                  return QJ.addClass(this.$card, 'jp-card-flipped')
                },
                unflipCard: function () {
                  return QJ.removeClass(this.$card, 'jp-card-flipped')
                }
              };
              bindVal = function (el, out, opts) {
                var joiner, o, outDefaults;
                if (opts == null) {
                  opts = {}
                }
                opts.fill = opts.fill || false;
                opts.filters = opts.filters || [];
                if (!(opts.filters instanceof Array)) {
                  opts.filters = [opts.filters]
                }
                opts.join = opts.join || '';
                if (!(typeof opts.join === 'function')) {
                  joiner = opts.join;
                  opts.join = function () {
                    return joiner
                  }
                }
                outDefaults = function () {
                  var _i, _len, _results;
                  _results = [];
                  for (_i = 0, _len = out.length; _i < _len; _i++) {
                    o = out[_i];
                    _results.push(o.textContent)
                  }
                  return _results
                }();
                QJ.on(el, 'focus', function () {
                  return QJ.addClass(out, 'jp-card-focused')
                });
                QJ.on(el, 'blur', function () {
                  return QJ.removeClass(out, 'jp-card-focused')
                });
                QJ.on(el, 'keyup change paste', function (e) {
                  var elem, filter, i, join, outEl, outVal, val, _i, _j, _len, _len1, _ref, _results;
                  val = function () {
                    var _i, _len, _results;
                    _results = [];
                    for (_i = 0, _len = el.length; _i < _len; _i++) {
                      elem = el[_i];
                      _results.push(QJ.val(elem))
                    }
                    return _results
                  }();
                  join = opts.join(val);
                  val = val.join(join);
                  if (val === join) {
                    val = ''
                  }
                  _ref = opts.filters;
                  for (_i = 0, _len = _ref.length; _i < _len; _i++) {
                    filter = _ref[_i];
                    val = filter(val, el, out)
                  }
                  _results = [];
                  for (i = _j = 0, _len1 = out.length; _j < _len1; i = ++_j) {
                    outEl = out[i];
                    if (opts.fill) {
                      outVal = val + outDefaults[i].substring(val.length)
                    } else {
                      outVal = val || outDefaults[i]
                    }
                    _results.push(outEl.textContent = outVal)
                  }
                  return _results
                });
                return el
              };
              return Card
            }();
            module.exports = Card;
            global.Card = Card
          }.call(this, typeof global !== 'undefined' ? global : typeof self !== 'undefined' ? self : typeof window !== 'undefined' ? window : {}))
        },
        {
          '../scss/card.scss': 9,
          './payment/src/payment.coffee': 8,
          'node.extend': 1,
          'qj': 4
        }
      ],
      8: [
        function (require, module, exports) {
          (function (global) {
            var Payment, QJ, cardFromNumber, cardFromType, cards, defaultFormat, formatBackCardNumber, formatBackExpiry, formatCardNumber, formatExpiry, formatForwardExpiry, formatForwardSlash, hasTextSelected, luhnCheck, reFormatCardNumber, restrictCVC, restrictCardNumber, restrictExpiry, restrictNumeric, setCardType, __indexOf = [].indexOf || function (item) {
                for (var i = 0, l = this.length; i < l; i++) {
                  if (i in this && this[i] === item)
                    return i
                }
                return -1
              };
            QJ = require('qj');
            defaultFormat = /(\d{1,4})/g;
            cards = [
              {
                type: 'amex',
                pattern: /^3[47]/,
                format: /(\d{1,4})(\d{1,6})?(\d{1,5})?/,
                length: [15],
                cvcLength: [4],
                luhn: true
              },
              {
                type: 'dankort',
                pattern: /^5019/,
                format: defaultFormat,
                length: [16],
                cvcLength: [3],
                luhn: true
              },
              {
                type: 'dinersclub',
                pattern: /^(36|38|30[0-5])/,
                format: defaultFormat,
                length: [14],
                cvcLength: [3],
                luhn: true
              },
              {
                type: 'discover',
                pattern: /^(6011|65|64[4-9]|622)/,
                format: defaultFormat,
                length: [16],
                cvcLength: [3],
                luhn: true
              },
              {
                type: 'jcb',
                pattern: /^35/,
                format: defaultFormat,
                length: [16],
                cvcLength: [3],
                luhn: true
              },
              {
                type: 'laser',
                pattern: /^(6706|6771|6709)/,
                format: defaultFormat,
                length: [
                  16,
                  17,
                  18,
                  19
                ],
                cvcLength: [3],
                luhn: true
              },
              {
                type: 'maestro',
                pattern: /^(5018|5020|5038|6304|6703|6759|676[1-3])/,
                format: defaultFormat,
                length: [
                  12,
                  13,
                  14,
                  15,
                  16,
                  17,
                  18,
                  19
                ],
                cvcLength: [3],
                luhn: true
              },
              {
                type: 'mastercard',
                pattern: /^5[1-5]/,
                format: defaultFormat,
                length: [16],
                cvcLength: [3],
                luhn: true
              },
              {
                type: 'unionpay',
                pattern: /^62/,
                format: defaultFormat,
                length: [
                  16,
                  17,
                  18,
                  19
                ],
                cvcLength: [3],
                luhn: false
              },
              {
                type: 'visaelectron',
                pattern: /^4(026|17500|405|508|844|91[37])/,
                format: defaultFormat,
                length: [16],
                cvcLength: [3],
                luhn: true
              },
              {
                type: 'visa',
                pattern: /^4/,
                format: defaultFormat,
                length: [
                  13,
                  14,
                  15,
                  16
                ],
                cvcLength: [3],
                luhn: true
              }
            ];
            cardFromNumber = function (num) {
              var card, _i, _len;
              num = (num + '').replace(/\D/g, '');
              for (_i = 0, _len = cards.length; _i < _len; _i++) {
                card = cards[_i];
                if (card.pattern.test(num)) {
                  return card
                }
              }
            };
            cardFromType = function (type) {
              var card, _i, _len;
              for (_i = 0, _len = cards.length; _i < _len; _i++) {
                card = cards[_i];
                if (card.type === type) {
                  return card
                }
              }
            };
            luhnCheck = function (num) {
              var digit, digits, odd, sum, _i, _len;
              odd = true;
              sum = 0;
              digits = (num + '').split('').reverse();
              for (_i = 0, _len = digits.length; _i < _len; _i++) {
                digit = digits[_i];
                digit = parseInt(digit, 10);
                if (odd = !odd) {
                  digit *= 2
                }
                if (digit > 9) {
                  digit -= 9
                }
                sum += digit
              }
              return sum % 10 === 0
            };
            hasTextSelected = function (target) {
              var _ref;
              if (target.selectionStart != null && target.selectionStart !== target.selectionEnd) {
                return true
              }
              if ((typeof document !== 'undefined' && document !== null ? (_ref = document.selection) != null ? _ref.createRange : void 0 : void 0) != null) {
                if (document.selection.createRange().text) {
                  return true
                }
              }
              return false
            };
            reFormatCardNumber = function (e) {
              return setTimeout(function (_this) {
                return function () {
                  var target, value;
                  target = e.target;
                  value = QJ.val(target);
                  value = Payment.fns.formatCardNumber(value);
                  return QJ.val(target, value)
                }
              }(this))
            };
            formatCardNumber = function (e) {
              var card, digit, length, re, target, upperLength, value;
              digit = String.fromCharCode(e.which);
              if (!/^\d+$/.test(digit)) {
                return
              }
              target = e.target;
              value = QJ.val(target);
              card = cardFromNumber(value + digit);
              length = (value.replace(/\D/g, '') + digit).length;
              upperLength = 16;
              if (card) {
                upperLength = card.length[card.length.length - 1]
              }
              if (length >= upperLength) {
                return
              }
              if (target.selectionStart != null && target.selectionStart !== value.length) {
                return
              }
              if (card && card.type === 'amex') {
                re = /^(\d{4}|\d{4}\s\d{6})$/
              } else {
                re = /(?:^|\s)(\d{4})$/
              }
              if (re.test(value)) {
                e.preventDefault();
                return QJ.val(target, value + ' ' + digit)
              } else if (re.test(value + digit)) {
                e.preventDefault();
                return QJ.val(target, value + digit + ' ')
              }
            };
            formatBackCardNumber = function (e) {
              var target, value;
              target = e.target;
              value = QJ.val(target);
              if (e.meta) {
                return
              }
              if (e.which !== 8) {
                return
              }
              if (target.selectionStart != null && target.selectionStart !== value.length) {
                return
              }
              if (/\d\s$/.test(value)) {
                e.preventDefault();
                return QJ.val(target, value.replace(/\d\s$/, ''))
              } else if (/\s\d?$/.test(value)) {
                e.preventDefault();
                return QJ.val(target, value.replace(/\s\d?$/, ''))
              }
            };
            formatExpiry = function (e) {
              var digit, target, val;
              digit = String.fromCharCode(e.which);
              if (!/^\d+$/.test(digit)) {
                return
              }
              target = e.target;
              val = QJ.val(target) + digit;
              if (/^\d$/.test(val) && (val !== '0' && val !== '1')) {
                e.preventDefault();
                return QJ.val(target, '0' + val + ' / ')
              } else if (/^\d\d$/.test(val)) {
                e.preventDefault();
                return QJ.val(target, '' + val + ' / ')
              }
            };
            formatForwardExpiry = function (e) {
              var digit, target, val;
              digit = String.fromCharCode(e.which);
              if (!/^\d+$/.test(digit)) {
                return
              }
              target = e.target;
              val = QJ.val(target);
              if (/^\d\d$/.test(val)) {
                return QJ.val(target, '' + val + ' / ')
              }
            };
            formatForwardSlash = function (e) {
              var slash, target, val;
              slash = String.fromCharCode(e.which);
              if (slash !== '/') {
                return
              }
              target = e.target;
              val = QJ.val(target);
              if (/^\d$/.test(val) && val !== '0') {
                return QJ.val(target, '0' + val + ' / ')
              }
            };
            formatBackExpiry = function (e) {
              var target, value;
              if (e.metaKey) {
                return
              }
              target = e.target;
              value = QJ.val(target);
              if (e.which !== 8) {
                return
              }
              if (target.selectionStart != null && target.selectionStart !== value.length) {
                return
              }
              if (/\d(\s|\/)+$/.test(value)) {
                e.preventDefault();
                return QJ.val(target, value.replace(/\d(\s|\/)*$/, ''))
              } else if (/\s\/\s?\d?$/.test(value)) {
                e.preventDefault();
                return QJ.val(target, value.replace(/\s\/\s?\d?$/, ''))
              }
            };
            restrictNumeric = function (e) {
              var input;
              if (e.metaKey || e.ctrlKey) {
                return true
              }
              if (e.which === 32) {
                return e.preventDefault()
              }
              if (e.which === 0) {
                return true
              }
              if (e.which < 33) {
                return true
              }
              input = String.fromCharCode(e.which);
              if (!/[\d\s]/.test(input)) {
                return e.preventDefault()
              }
            };
            restrictCardNumber = function (e) {
              var card, digit, target, value;
              target = e.target;
              digit = String.fromCharCode(e.which);
              if (!/^\d+$/.test(digit)) {
                return
              }
              if (hasTextSelected(target)) {
                return
              }
              value = (QJ.val(target) + digit).replace(/\D/g, '');
              card = cardFromNumber(value);
              if (card) {
                if (!(value.length <= card.length[card.length.length - 1])) {
                  return e.preventDefault()
                }
              } else {
                if (!(value.length <= 16)) {
                  return e.preventDefault()
                }
              }
            };
            restrictExpiry = function (e) {
              var digit, target, value;
              target = e.target;
              digit = String.fromCharCode(e.which);
              if (!/^\d+$/.test(digit)) {
                return
              }
              if (hasTextSelected(target)) {
                return
              }
              value = QJ.val(target) + digit;
              value = value.replace(/\D/g, '');
              if (value.length > 6) {
                return e.preventDefault()
              }
            };
            restrictCVC = function (e) {
              var digit, target, val;
              target = e.target;
              digit = String.fromCharCode(e.which);
              if (!/^\d+$/.test(digit)) {
                return
              }
              val = QJ.val(target) + digit;
              if (!(val.length <= 4)) {
                return e.preventDefault()
              }
            };
            setCardType = function (e) {
              var allTypes, card, cardType, target, val;
              target = e.target;
              val = QJ.val(target);
              cardType = Payment.fns.cardType(val) || 'unknown';
              if (!QJ.hasClass(target, cardType)) {
                allTypes = function () {
                  var _i, _len, _results;
                  _results = [];
                  for (_i = 0, _len = cards.length; _i < _len; _i++) {
                    card = cards[_i];
                    _results.push(card.type)
                  }
                  return _results
                }();
                QJ.removeClass(target, 'unknown');
                QJ.removeClass(target, allTypes.join(' '));
                QJ.addClass(target, cardType);
                QJ.toggleClass(target, 'identified', cardType !== 'unknown');
                return QJ.trigger(target, 'payment.cardType', cardType)
              }
            };
            Payment = function () {
              function Payment() {
              }
              Payment.fns = {
                cardExpiryVal: function (value) {
                  var month, prefix, year, _ref;
                  value = value.replace(/\s/g, '');
                  _ref = value.split('/', 2), month = _ref[0], year = _ref[1];
                  if ((year != null ? year.length : void 0) === 2 && /^\d+$/.test(year)) {
                    prefix = new Date().getFullYear();
                    prefix = prefix.toString().slice(0, 2);
                    year = prefix + year
                  }
                  month = parseInt(month, 10);
                  year = parseInt(year, 10);
                  return {
                    month: month,
                    year: year
                  }
                },
                validateCardNumber: function (num) {
                  var card, _ref;
                  num = (num + '').replace(/\s+|-/g, '');
                  if (!/^\d+$/.test(num)) {
                    return false
                  }
                  card = cardFromNumber(num);
                  if (!card) {
                    return false
                  }
                  return (_ref = num.length, __indexOf.call(card.length, _ref) >= 0) && (card.luhn === false || luhnCheck(num))
                },
                validateCardExpiry: function (month, year) {
                  var currentTime, expiry, prefix, _ref;
                  if (typeof month === 'object' && 'month' in month) {
                    _ref = month, month = _ref.month, year = _ref.year
                  }
                  if (!(month && year)) {
                    return false
                  }
                  month = QJ.trim(month);
                  year = QJ.trim(year);
                  if (!/^\d+$/.test(month)) {
                    return false
                  }
                  if (!/^\d+$/.test(year)) {
                    return false
                  }
                  if (!(parseInt(month, 10) <= 12)) {
                    return false
                  }
                  if (year.length === 2) {
                    prefix = new Date().getFullYear();
                    prefix = prefix.toString().slice(0, 2);
                    year = prefix + year
                  }
                  expiry = new Date(year, month);
                  currentTime = new Date;
                  expiry.setMonth(expiry.getMonth() - 1);
                  expiry.setMonth(expiry.getMonth() + 1, 1);
                  return expiry > currentTime
                },
                validateCardCVC: function (cvc, type) {
                  var _ref, _ref1;
                  cvc = QJ.trim(cvc);
                  if (!/^\d+$/.test(cvc)) {
                    return false
                  }
                  if (type && cardFromType(type)) {
                    return _ref = cvc.length, __indexOf.call((_ref1 = cardFromType(type)) != null ? _ref1.cvcLength : void 0, _ref) >= 0
                  } else {
                    return cvc.length >= 3 && cvc.length <= 4
                  }
                },
                cardType: function (num) {
                  var _ref;
                  if (!num) {
                    return null
                  }
                  return ((_ref = cardFromNumber(num)) != null ? _ref.type : void 0) || null
                },
                formatCardNumber: function (num) {
                  var card, groups, upperLength, _ref;
                  card = cardFromNumber(num);
                  if (!card) {
                    return num
                  }
                  upperLength = card.length[card.length.length - 1];
                  num = num.replace(/\D/g, '');
                  num = num.slice(0, +upperLength + 1 || 9000000000);
                  if (card.format.global) {
                    return (_ref = num.match(card.format)) != null ? _ref.join(' ') : void 0
                  } else {
                    groups = card.format.exec(num);
                    if (groups != null) {
                      groups.shift()
                    }
                    return groups != null ? groups.join(' ') : void 0
                  }
                }
              };
              Payment.restrictNumeric = function (el) {
                return QJ.on(el, 'keypress', restrictNumeric)
              };
              Payment.cardExpiryVal = function (el) {
                return Payment.fns.cardExpiryVal(QJ.val(el))
              };
              Payment.formatCardCVC = function (el) {
                Payment.restrictNumeric(el);
                QJ.on(el, 'keypress', restrictCVC);
                return el
              };
              Payment.formatCardExpiry = function (el) {
                Payment.restrictNumeric(el);
                QJ.on(el, 'keypress', restrictExpiry);
                QJ.on(el, 'keypress', formatExpiry);
                QJ.on(el, 'keypress', formatForwardSlash);
                QJ.on(el, 'keypress', formatForwardExpiry);
                QJ.on(el, 'keydown', formatBackExpiry);
                return el
              };
              Payment.formatCardNumber = function (el) {
                Payment.restrictNumeric(el);
                QJ.on(el, 'keypress', restrictCardNumber);
                QJ.on(el, 'keypress', formatCardNumber);
                QJ.on(el, 'keydown', formatBackCardNumber);
                QJ.on(el, 'keyup', setCardType);
                QJ.on(el, 'paste', reFormatCardNumber);
                return el
              };
              Payment.getCardArray = function () {
                return cards
              };
              Payment.setCardArray = function (cardArray) {
                cards = cardArray;
                return true
              };
              Payment.addToCardArray = function (cardObject) {
                return cards.push(cardObject)
              };
              Payment.removeFromCardArray = function (type) {
                var key, value;
                for (key in cards) {
                  value = cards[key];
                  if (value.type === type) {
                    cards.splice(key, 1)
                  }
                }
                return true
              };
              return Payment
            }();
            module.exports = Payment;
            global.Payment = Payment
          }.call(this, typeof global !== 'undefined' ? global : typeof self !== 'undefined' ? self : typeof window !== 'undefined' ? window : {}))
        },
        { 'qj': 4 }
      ],
      9: [
        function (require, module, exports) {
          module.exports = require('sassify')('.jp-card.jp-card-safari.jp-card-identified .jp-card-front:before, .jp-card.jp-card-safari.jp-card-identified .jp-card-back:before {   background-image: repeating-linear-gradient(45deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-linear-gradient(135deg, rgba(255, 255, 255, 0.05) 1px, rgba(255, 255, 255, 0) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.03) 4px), repeating-linear-gradient(90deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-linear-gradient(210deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), -webkit-linear-gradient(-245deg, rgba(255, 255, 255, 0) 50%, rgba(255, 255, 255, 0.2) 70%, rgba(255, 255, 255, 0) 90%);   background-image: repeating-linear-gradient(45deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-linear-gradient(135deg, rgba(255, 255, 255, 0.05) 1px, rgba(255, 255, 255, 0) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.03) 4px), repeating-linear-gradient(90deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-linear-gradient(210deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), linear-gradient(-25deg, rgba(255, 255, 255, 0) 50%, rgba(255, 255, 255, 0.2) 70%, rgba(255, 255, 255, 0) 90%); }  .jp-card.jp-card-ie-10.jp-card-flipped, .jp-card.jp-card-ie-11.jp-card-flipped {   -webkit-transform: 0deg;   -moz-transform: 0deg;   -ms-transform: 0deg;   -o-transform: 0deg;   transform: 0deg; }   .jp-card.jp-card-ie-10.jp-card-flipped .jp-card-front, .jp-card.jp-card-ie-11.jp-card-flipped .jp-card-front {     -webkit-transform: rotateY(0deg);     -moz-transform: rotateY(0deg);     -ms-transform: rotateY(0deg);     -o-transform: rotateY(0deg);     transform: rotateY(0deg); }   .jp-card.jp-card-ie-10.jp-card-flipped .jp-card-back, .jp-card.jp-card-ie-11.jp-card-flipped .jp-card-back {     -webkit-transform: rotateY(0deg);     -moz-transform: rotateY(0deg);     -ms-transform: rotateY(0deg);     -o-transform: rotateY(0deg);     transform: rotateY(0deg); }     .jp-card.jp-card-ie-10.jp-card-flipped .jp-card-back:after, .jp-card.jp-card-ie-11.jp-card-flipped .jp-card-back:after {       left: 18%; }     .jp-card.jp-card-ie-10.jp-card-flipped .jp-card-back .jp-card-cvc, .jp-card.jp-card-ie-11.jp-card-flipped .jp-card-back .jp-card-cvc {       -webkit-transform: rotateY(180deg);       -moz-transform: rotateY(180deg);       -ms-transform: rotateY(180deg);       -o-transform: rotateY(180deg);       transform: rotateY(180deg);       left: 5%; }     .jp-card.jp-card-ie-10.jp-card-flipped .jp-card-back .jp-card-shiny, .jp-card.jp-card-ie-11.jp-card-flipped .jp-card-back .jp-card-shiny {       left: 84%; }       .jp-card.jp-card-ie-10.jp-card-flipped .jp-card-back .jp-card-shiny:after, .jp-card.jp-card-ie-11.jp-card-flipped .jp-card-back .jp-card-shiny:after {         left: -480%;         -webkit-transform: rotateY(180deg);         -moz-transform: rotateY(180deg);         -ms-transform: rotateY(180deg);         -o-transform: rotateY(180deg);         transform: rotateY(180deg); }  .jp-card.jp-card-ie-10.jp-card-amex .jp-card-back, .jp-card.jp-card-ie-11.jp-card-amex .jp-card-back {   display: none; }  .jp-card-logo {   height: 36px;   width: 60px;   font-style: italic; }   .jp-card-logo, .jp-card-logo:before, .jp-card-logo:after {     box-sizing: border-box; }  .jp-card-logo.jp-card-amex {   text-transform: uppercase;   font-size: 4px;   font-weight: bold;   color: white;   background-image: repeating-radial-gradient(circle at center, #FFF 1px, #999 2px);   background-image: repeating-radial-gradient(circle at center, #FFF 1px, #999 2px);   border: 1px solid #EEE; }   .jp-card-logo.jp-card-amex:before, .jp-card-logo.jp-card-amex:after {     width: 28px;     display: block;     position: absolute;     left: 16px; }   .jp-card-logo.jp-card-amex:before {     height: 28px;     content: "american";     top: 3px;     text-align: left;     padding-left: 2px;     padding-top: 11px;     background: #267AC3; }   .jp-card-logo.jp-card-amex:after {     content: "express";     bottom: 11px;     text-align: right;     padding-right: 2px; }  .jp-card.jp-card-amex.jp-card-flipped {   -webkit-transform: none;   -moz-transform: none;   -ms-transform: none;   -o-transform: none;   transform: none; }  .jp-card.jp-card-amex.jp-card-identified .jp-card-front:before, .jp-card.jp-card-amex.jp-card-identified .jp-card-back:before {   background-color: #108168; }  .jp-card.jp-card-amex.jp-card-identified .jp-card-front .jp-card-logo.jp-card-amex {   opacity: 1; }  .jp-card.jp-card-amex.jp-card-identified .jp-card-front .jp-card-cvc {   visibility: visible; }  .jp-card.jp-card-amex.jp-card-identified .jp-card-front:after {   opacity: 1; }  .jp-card-logo.jp-card-discover {   background: #FF6600;   color: #111;   text-transform: uppercase;   font-style: normal;   font-weight: bold;   font-size: 10px;   text-align: center;   overflow: hidden;   z-index: 1;   padding-top: 9px;   letter-spacing: .03em;   border: 1px solid #EEE; }   .jp-card-logo.jp-card-discover:before, .jp-card-logo.jp-card-discover:after {     content: " ";     display: block;     position: absolute; }   .jp-card-logo.jp-card-discover:before {     background: white;     width: 200px;     height: 200px;     border-radius: 200px;     bottom: -5%;     right: -80%;     z-index: -1; }   .jp-card-logo.jp-card-discover:after {     width: 8px;     height: 8px;     border-radius: 4px;     top: 10px;     left: 27px;     background-color: #FF6600;     background-image: -webkit-radial-gradient(#FF6600, #fff, , , , , , , , );     background-image: radial-gradient(  #FF6600, #fff, , , , , , , , );     content: "network";     font-size: 4px;     line-height: 24px;     text-indent: -7px; }  .jp-card .jp-card-front .jp-card-logo.jp-card-discover {   right: 12%;   top: 18%; }  .jp-card.jp-card-discover.jp-card-identified .jp-card-front:before, .jp-card.jp-card-discover.jp-card-identified .jp-card-back:before {   background-color: #86B8CF; }  .jp-card.jp-card-discover.jp-card-identified .jp-card-logo.jp-card-discover {   opacity: 1; }  .jp-card.jp-card-discover.jp-card-identified .jp-card-front:after {   -webkit-transition: 400ms;   -moz-transition: 400ms;   transition: 400ms;   content: " ";   display: block;   background-color: #FF6600;   background-image: -webkit-linear-gradient(#FF6600, #ffa366, #FF6600);   background-image: linear-gradient(#FF6600, #ffa366, #FF6600, , , , , , , );   height: 50px;   width: 50px;   border-radius: 25px;   position: absolute;   left: 100%;   top: 15%;   margin-left: -25px;   box-shadow: inset 1px 1px 3px 1px rgba(0, 0, 0, 0.5); }  .jp-card-logo.jp-card-visa {   background: white;   text-transform: uppercase;   color: #1A1876;   text-align: center;   font-weight: bold;   font-size: 15px;   line-height: 18px; }   .jp-card-logo.jp-card-visa:before, .jp-card-logo.jp-card-visa:after {     content: " ";     display: block;     width: 100%;     height: 25%; }   .jp-card-logo.jp-card-visa:before {     background: #1A1876; }   .jp-card-logo.jp-card-visa:after {     background: #E79800; }  .jp-card.jp-card-visa.jp-card-identified .jp-card-front:before, .jp-card.jp-card-visa.jp-card-identified .jp-card-back:before {   background-color: #191278; }  .jp-card.jp-card-visa.jp-card-identified .jp-card-logo.jp-card-visa {   opacity: 1; }  .jp-card-logo.jp-card-mastercard {   color: white;   font-weight: bold;   text-align: center;   font-size: 9px;   line-height: 36px;   z-index: 1;   text-shadow: 1px 1px rgba(0, 0, 0, 0.6); }   .jp-card-logo.jp-card-mastercard:before, .jp-card-logo.jp-card-mastercard:after {     content: " ";     display: block;     width: 36px;     top: 0;     position: absolute;     height: 36px;     border-radius: 18px; }   .jp-card-logo.jp-card-mastercard:before {     left: 0;     background: #FF0000;     z-index: -1; }   .jp-card-logo.jp-card-mastercard:after {     right: 0;     background: #FFAB00;     z-index: -2; }  .jp-card.jp-card-mastercard.jp-card-identified .jp-card-front .jp-card-logo.jp-card-mastercard, .jp-card.jp-card-mastercard.jp-card-identified .jp-card-back .jp-card-logo.jp-card-mastercard {   box-shadow: none; }  .jp-card.jp-card-mastercard.jp-card-identified .jp-card-front:before, .jp-card.jp-card-mastercard.jp-card-identified .jp-card-back:before {   background-color: #0061A8; }  .jp-card.jp-card-mastercard.jp-card-identified .jp-card-logo.jp-card-mastercard {   opacity: 1; }  .jp-card-logo.jp-card-maestro {   color: white;   font-weight: bold;   text-align: center;   font-size: 14px;   line-height: 36px;   z-index: 1;   text-shadow: 1px 1px rgba(0, 0, 0, 0.6); }   .jp-card-logo.jp-card-maestro:before, .jp-card-logo.jp-card-maestro:after {     content: " ";     display: block;     width: 36px;     top: 0;     position: absolute;     height: 36px;     border-radius: 18px; }   .jp-card-logo.jp-card-maestro:before {     left: 0;     background: #0064CB;     z-index: -1; }   .jp-card-logo.jp-card-maestro:after {     right: 0;     background: #CC0000;     z-index: -2; }  .jp-card.jp-card-maestro.jp-card-identified .jp-card-front .jp-card-logo.jp-card-maestro, .jp-card.jp-card-maestro.jp-card-identified .jp-card-back .jp-card-logo.jp-card-maestro {   box-shadow: none; }  .jp-card.jp-card-maestro.jp-card-identified .jp-card-front:before, .jp-card.jp-card-maestro.jp-card-identified .jp-card-back:before {   background-color: #0B2C5F; }  .jp-card.jp-card-maestro.jp-card-identified .jp-card-logo.jp-card-maestro {   opacity: 1; }  .jp-card-logo.jp-card-dankort {   width: 60px;   height: 36px;   padding: 3px;   border-radius: 8px;   border: #000000 1px solid;   background-color: #FFFFFF; }   .jp-card-logo.jp-card-dankort .dk {     position: relative;     width: 100%;     height: 100%;     overflow: hidden; }     .jp-card-logo.jp-card-dankort .dk:before {       background-color: #ED1C24;       content: \'\';       position: absolute;       width: 100%;       height: 100%;       display: block;       border-radius: 6px; }     .jp-card-logo.jp-card-dankort .dk:after {       content: \'\';       position: absolute;       top: 50%;       margin-top: -7.7px;       right: 0;       width: 0;       height: 0;       border-style: solid;       border-width: 7px 7px 10px 0;       border-color: transparent #ED1C24 transparent transparent;       z-index: 1; }   .jp-card-logo.jp-card-dankort .d, .jp-card-logo.jp-card-dankort .k {     position: absolute;     top: 50%;     width: 50%;     display: block;     height: 15.4px;     margin-top: -7.7px;     background: white; }   .jp-card-logo.jp-card-dankort .d {     left: 0;     border-radius: 0 8px 10px 0; }     .jp-card-logo.jp-card-dankort .d:before {       content: \'\';       position: absolute;       top: 50%;       left: 50%;       display: block;       background: #ED1C24;       border-radius: 2px 4px 6px 0px;       height: 5px;       width: 7px;       margin: -3px 0 0 -4px; }   .jp-card-logo.jp-card-dankort .k {     right: 0; }     .jp-card-logo.jp-card-dankort .k:before, .jp-card-logo.jp-card-dankort .k:after {       content: \'\';       position: absolute;       right: 50%;       width: 0;       height: 0;       border-style: solid;       margin-right: -1px; }     .jp-card-logo.jp-card-dankort .k:before {       top: 0;       border-width: 8px 5px 0 0;       border-color: #ED1C24 transparent transparent transparent; }     .jp-card-logo.jp-card-dankort .k:after {       bottom: 0;       border-width: 0 5px 8px 0;       border-color: transparent transparent #ED1C24 transparent; }  .jp-card.jp-card-dankort.jp-card-identified .jp-card-front:before, .jp-card.jp-card-dankort.jp-card-identified .jp-card-back:before {   background-color: #0055C7; }  .jp-card.jp-card-dankort.jp-card-identified .jp-card-logo.jp-card-dankort {   opacity: 1; }  .jp-card-container {   -webkit-perspective: 1000px;   -moz-perspective: 1000px;   perspective: 1000px;   width: 350px;   max-width: 100%;   height: 200px;   margin: auto;   z-index: 1;   position: relative; }  .jp-card {   font-family: "Helvetica Neue";   line-height: 1;   position: relative;   width: 100%;   height: 100%;   min-width: 315px;   border-radius: 10px;   -webkit-transform-style: preserve-3d;   -moz-transform-style: preserve-3d;   -ms-transform-style: preserve-3d;   -o-transform-style: preserve-3d;   transform-style: preserve-3d;   -webkit-transition: all 400ms linear;   -moz-transition: all 400ms linear;   transition: all 400ms linear; }   .jp-card > *, .jp-card > *:before, .jp-card > *:after {     -moz-box-sizing: border-box;     -webkit-box-sizing: border-box;     box-sizing: border-box;     font-family: inherit; }   .jp-card.jp-card-flipped {     -webkit-transform: rotateY(180deg);     -moz-transform: rotateY(180deg);     -ms-transform: rotateY(180deg);     -o-transform: rotateY(180deg);     transform: rotateY(180deg); }   .jp-card .jp-card-front, .jp-card .jp-card-back {     -webkit-backface-visibility: hidden;     backface-visibility: hidden;     -webkit-transform-style: preserve-3d;     -moz-transform-style: preserve-3d;     -ms-transform-style: preserve-3d;     -o-transform-style: preserve-3d;     transform-style: preserve-3d;     -webkit-transition: all 400ms linear;     -moz-transition: all 400ms linear;     transition: all 400ms linear;     width: 100%;     height: 100%;     position: absolute;     top: 0;     left: 0;     overflow: hidden;     border-radius: 10px;     background: #DDD; }     .jp-card .jp-card-front:before, .jp-card .jp-card-back:before {       content: " ";       display: block;       position: absolute;       width: 100%;       height: 100%;       top: 0;       left: 0;       opacity: 0;       border-radius: 10px;       -webkit-transition: all 400ms ease;       -moz-transition: all 400ms ease;       transition: all 400ms ease; }     .jp-card .jp-card-front:after, .jp-card .jp-card-back:after {       content: " ";       display: block; }     .jp-card .jp-card-front .jp-card-display, .jp-card .jp-card-back .jp-card-display {       color: white;       font-weight: normal;       opacity: 0.5;       -webkit-transition: opacity 400ms linear;       -moz-transition: opacity 400ms linear;       transition: opacity 400ms linear; }       .jp-card .jp-card-front .jp-card-display.jp-card-focused, .jp-card .jp-card-back .jp-card-display.jp-card-focused {         opacity: 1;         font-weight: 700; }     .jp-card .jp-card-front .jp-card-cvc, .jp-card .jp-card-back .jp-card-cvc {       font-family: "Bitstream Vera Sans Mono", Consolas, Courier, monospace;       font-size: 14px; }     .jp-card .jp-card-front .jp-card-shiny, .jp-card .jp-card-back .jp-card-shiny {       width: 50px;       height: 35px;       border-radius: 5px;       background: #CCC;       position: relative; }       .jp-card .jp-card-front .jp-card-shiny:before, .jp-card .jp-card-back .jp-card-shiny:before {         content: " ";         display: block;         width: 70%;         height: 60%;         border-top-right-radius: 5px;         border-bottom-right-radius: 5px;         background: #d9d9d9;         position: absolute;         top: 20%; }   .jp-card .jp-card-front .jp-card-logo {     position: absolute;     opacity: 0;     right: 5%;     top: 8%;     -webkit-transition: 400ms;     -moz-transition: 400ms;     transition: 400ms; }   .jp-card .jp-card-front .jp-card-lower {     width: 80%;     position: absolute;     left: 10%;     bottom: 30px; }     @media only screen and (max-width: 480px) {       .jp-card .jp-card-front .jp-card-lower {         width: 90%;         left: 5%; } }     .jp-card .jp-card-front .jp-card-lower .jp-card-cvc {       visibility: hidden;       float: right;       position: relative;       bottom: 5px; }     .jp-card .jp-card-front .jp-card-lower .jp-card-number {       font-family: "Bitstream Vera Sans Mono", Consolas, Courier, monospace;       font-size: 24px;       clear: both;       margin-bottom: 30px; }     .jp-card .jp-card-front .jp-card-lower .jp-card-expiry {       font-family: "Bitstream Vera Sans Mono", Consolas, Courier, monospace;       letter-spacing: 0em;       position: relative;       float: right;       width: 25%; }       .jp-card .jp-card-front .jp-card-lower .jp-card-expiry:before, .jp-card .jp-card-front .jp-card-lower .jp-card-expiry:after {         font-family: "Helvetica Neue";         font-weight: bold;         font-size: 7px;         white-space: pre;         display: block;         opacity: .5; }       .jp-card .jp-card-front .jp-card-lower .jp-card-expiry:before {         content: attr(data-before);         margin-bottom: 2px;         font-size: 7px;         text-transform: uppercase; }       .jp-card .jp-card-front .jp-card-lower .jp-card-expiry:after {         position: absolute;         content: attr(data-after);         text-align: right;         right: 100%;         margin-right: 5px;         margin-top: 2px;         bottom: 0; }     .jp-card .jp-card-front .jp-card-lower .jp-card-name {       text-transform: uppercase;       font-family: "Bitstream Vera Sans Mono", Consolas, Courier, monospace;       font-size: 20px;       max-height: 45px;       position: absolute;       bottom: 0;       width: 190px;       display: -webkit-box;       -webkit-line-clamp: 2;       -webkit-box-orient: horizontal;       overflow: hidden;       text-overflow: ellipsis; }   .jp-card .jp-card-back {     -webkit-transform: rotateY(180deg);     -moz-transform: rotateY(180deg);     -ms-transform: rotateY(180deg);     -o-transform: rotateY(180deg);     transform: rotateY(180deg); }     .jp-card .jp-card-back .jp-card-bar {       background-color: #444;       background-image: -webkit-linear-gradient(#444, #333);       background-image: linear-gradient(#444, #333, , , , , , , , );       width: 100%;       height: 20%;       position: absolute;       top: 10%; }     .jp-card .jp-card-back:after {       content: " ";       display: block;       background-color: #FFF;       background-image: -webkit-linear-gradient(#FFF, #FFF);       background-image: linear-gradient(#FFF, #FFF, , , , , , , , );       width: 80%;       height: 16%;       position: absolute;       top: 40%;       left: 2%; }     .jp-card .jp-card-back .jp-card-cvc {       position: absolute;       top: 40%;       left: 85%;       -webkit-transition-delay: 600ms;       -moz-transition-delay: 600ms;       transition-delay: 600ms; }     .jp-card .jp-card-back .jp-card-shiny {       position: absolute;       top: 66%;       left: 2%; }       .jp-card .jp-card-back .jp-card-shiny:after {         content: "This card has been issued by Jesse Pollak and is licensed for anyone to use anywhere for free.AIt comes with no warranty.A For support issues, please visit: github.com/jessepollak/card.";         position: absolute;         left: 120%;         top: 5%;         color: white;         font-size: 7px;         width: 230px;         opacity: .5; }   .jp-card.jp-card-identified {     box-shadow: 0 0 20px rgba(0, 0, 0, 0.3); }     .jp-card.jp-card-identified .jp-card-front, .jp-card.jp-card-identified .jp-card-back {       background-color: #000;       background-color: rgba(0, 0, 0, 0.5); }       .jp-card.jp-card-identified .jp-card-front:before, .jp-card.jp-card-identified .jp-card-back:before {         -webkit-transition: all 400ms ease;         -moz-transition: all 400ms ease;         transition: all 400ms ease;         background-image: repeating-linear-gradient(45deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-linear-gradient(135deg, rgba(255, 255, 255, 0.05) 1px, rgba(255, 255, 255, 0) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.03) 4px), repeating-linear-gradient(90deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-linear-gradient(210deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-radial-gradient(circle at 30% 30%, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-radial-gradient(circle at 70% 70%, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-radial-gradient(circle at 90% 20%, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-radial-gradient(circle at 15% 80%, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), -webkit-linear-gradient(-245deg, rgba(255, 255, 255, 0) 50%, rgba(255, 255, 255, 0.2) 70%, rgba(255, 255, 255, 0) 90%);         background-image: repeating-linear-gradient(45deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-linear-gradient(135deg, rgba(255, 255, 255, 0.05) 1px, rgba(255, 255, 255, 0) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.03) 4px), repeating-linear-gradient(90deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-linear-gradient(210deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-radial-gradient(circle at 30% 30%, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-radial-gradient(circle at 70% 70%, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-radial-gradient(circle at 90% 20%, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-radial-gradient(circle at 15% 80%, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), linear-gradient(-25deg, rgba(255, 255, 255, 0) 50%, rgba(255, 255, 255, 0.2) 70%, rgba(255, 255, 255, 0) 90%);         opacity: 1; }       .jp-card.jp-card-identified .jp-card-front .jp-card-logo, .jp-card.jp-card-identified .jp-card-back .jp-card-logo {         box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.3); }     .jp-card.jp-card-identified.no-radial-gradient .jp-card-front:before, .jp-card.jp-card-identified.no-radial-gradient .jp-card-back:before {       background-image: repeating-linear-gradient(45deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-linear-gradient(135deg, rgba(255, 255, 255, 0.05) 1px, rgba(255, 255, 255, 0) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.03) 4px), repeating-linear-gradient(90deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-linear-gradient(210deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), -webkit-linear-gradient(-245deg, rgba(255, 255, 255, 0) 50%, rgba(255, 255, 255, 0.2) 70%, rgba(255, 255, 255, 0) 90%);       background-image: repeating-linear-gradient(45deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-linear-gradient(135deg, rgba(255, 255, 255, 0.05) 1px, rgba(255, 255, 255, 0) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.03) 4px), repeating-linear-gradient(90deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), repeating-linear-gradient(210deg, rgba(255, 255, 255, 0) 1px, rgba(255, 255, 255, 0.03) 2px, rgba(255, 255, 255, 0.04) 3px, rgba(255, 255, 255, 0.05) 4px), linear-gradient(-25deg, rgba(255, 255, 255, 0) 50%, rgba(255, 255, 255, 0.2) 70%, rgba(255, 255, 255, 0) 90%); } ');
          ;
        },
        { 'sassify': 5 }
      ]
    }, {}, [7]))
  });
  // source: /Users/dtai/work/verus/crowdstart/node_modules/address-validator/src/validator.coffee
  require.define('address-validator/src/validator', function (module, exports, __dirname, __filename) {
    var Address, _, addressMatch, defaultMatchType, matchType, matchUnknownType, options, request;
    _ = require('address-validator/node_modules/underscore/underscore');
    request = require('requisite/node_modules/browser-request');
    options = {
      countryBias: 'us',
      countryMatch: null,
      key: null
    };
    exports.setOptions = function (opts) {
      return _.extend(options, opts)
    };
    matchUnknownType = function (known, unknown) {
      var compare, find, found, foundProps, haveProps, i, len, otherAddress, prop, props, value;
      compare = function (_this) {
        return function (prop) {
          if (known[prop] && unknown[prop]) {
            if (known[prop].toLowerCase() === unknown[prop].toLowerCase()) {
              return true
            }
            if (unknown.generated && unknown[prop + 'Abbr']) {
              return known[prop].toLowerCase() === unknown[prop + 'Abbr'].toLowerCase()
            } else if (known.generated && known[prop + 'Abbr']) {
              return known[prop + 'Abbr'].toLowerCase() === unknown[prop].toLowerCase()
            } else {
              return false
            }
          }
          return !known[prop] && !unknown[prop]
        }
      }(this);
      if (known.isObject && unknown.isObject) {
        return compare('city') && compare('state') && compare('country')
      } else if (known.isObject && !unknown.isObject) {
        props = [
          'streetNumber',
          'street',
          'city',
          'state',
          'country',
          'postalCode'
        ];
        otherAddress = unknown.toString().toLowerCase();
        if (known.toString() === otherAddress) {
          return true
        }
        foundProps = 0;
        haveProps = 0;
        find = function (val) {
          var oldlen;
          val = val.toLowerCase();
          oldlen = otherAddress.length;
          otherAddress = otherAddress.replace(new RegExp('\\b' + val + '\\b', 'i'), '');
          if (oldlen !== otherAddress.length) {
            foundProps++;
            return true
          }
          return false
        };
        for (i = 0, len = props.length; i < len; i++) {
          prop = props[i];
          value = known[prop];
          if (value !== void 0) {
            found = find(value);
            if (!found && (prop === 'state' || prop === 'country' || prop === 'street') && known[prop + 'Abbr'] !== void 0) {
              found = find(known[prop + 'Abbr'])
            }
            if (!found && prop === 'country' && value.toLowerCase() === 'united states') {
              found = find('usa')
            }
            if (!found && prop === 'street') {
              value = value.replace(/( street)/i, ' st');
              found = find(value);
              if (!found) {
                value = value.replace(/( road)/i, ' rd');
                find(value)
              }
            }
            if (!found && prop === 'postalCode') {
              haveProps--
            }
            haveProps++
          }
        }
        otherAddress = otherAddress.replace(/[ ,]/g, '');
        return foundProps === haveProps && otherAddress.length === 0
      } else {
        return known.toString().toLowerCase() === unknown.toString().toLowerCase()
      }
    };
    addressMatch = {
      streetAddress: [
        {
          location_type: 'ROOFTOP',
          types: ['street_address'],
          exact: true
        },
        {
          location_type: 'RANGE_INTERPOLATED',
          types: ['street_address'],
          exact: false
        }
      ],
      route: [{
          location_type: 'GEOMETRIC_CENTER',
          types: ['route'],
          exact: true
        }],
      city: [{
          location_type: 'APPROXIMATE',
          types: [
            'locality',
            'political'
          ],
          exact: true
        }],
      state: [{
          location_type: 'APPROXIMATE',
          types: [
            'administrative_area_level_1',
            'political'
          ],
          exact: true
        }],
      country: [{
          location_type: 'APPROXIMATE',
          types: [
            'country',
            'political'
          ],
          exact: true
        }],
      unknown: [{
          location_type: 'unknown',
          types: ['unknown'],
          exact: true
        }]
    };
    exports.match = matchType = {};
    _.each(addressMatch, function (list, name) {
      return matchType[name] = name
    });
    /*
    Address object that provides useful methods. Create a new one by
      1. passing a map with these props: {street:'123 main st', city: 'boston', state: 'MA'|'massachussetts', country: 'US'|'United States'}
        None of the props are required, but chances are you wont have a valid address if you omit any of them (except for state)
      2. passing a string containing an address (the address class does not parse the string into parts)
      3. passing a result object from a google geocoding response. ie: geoResponse.results[0]


    The validator.validate callback will return to you these objects, except they will have all or some of the following properties:
        streetNumber: '100'
        street: 'North Main St'
        streetAbbr: 'N Main St'
        city: 'Boston'
        state: 'Massachussetts'
        stateAbbr: 'MA'
        country: 'United States'
        countryAbbr: 'US'
        postalCode: 02114
        location: {lat: 43.233332, lon: 23.2222243}

    Methods:
        toString(useCountryAbbr, useStateAbbr, useStreetAbbr) - returns a string representing the address. currently geared towards North American addresses
            useCountryAbbr = [optional] default: true - the resulting address string should use country abbr, not the full country name
            useStateAbbr   = [optional] default: true - the resulting address string should use state abbr, not the full state name
            useStreetAbbr  = [optional] default: false - the resulting address string should use street name abbr, not the full street name
            Note: the abbriviated values will only be used if they are available. The Address objects returned to you from the validate callback may have these available.
        equals(anotherAddress) - check if 2 addresses are probably* the same. IT DOES NOT CHECK STREET NAME/NUMBER
 */
    exports.Address = Address = function () {
      Address.prototype.matchType = matchType.unknown;
      Address.prototype.exactMatch = null;
      function Address(address, isObject, generated) {
        var city, country, countryAbbr, getComponent, location, postalCode, ref, ref1, ref2, ref3, ref4, ref5, ref6, ref7, ref8, ref9, state, stateAbbr, street, streetAbbr, streetNum, x;
        this.isObject = isObject != null ? isObject : false;
        this.generated = generated != null ? generated : false;
        if (_.isObject(address)) {
          this.isObject = true;
          if (address.address_components) {
            this.generated = true;
            location = {
              lat: (ref = address.geometry) != null ? (ref1 = ref.location) != null ? ref1.lat : void 0 : void 0,
              lon: (ref2 = address.geometry) != null ? (ref3 = ref2.location) != null ? ref3.lng : void 0 : void 0
            };
            this.exactMatch = !address.partial_match;
            _.each(addressMatch, function (_this) {
              return function (list, name) {
                return _.each(list, function (obj) {
                  if (obj.location_type === address.geometry.location_type && _.difference(obj.types, address.types).length === 0) {
                    _this.matchType = name;
                    if (!obj.exact) {
                      return _this.exactMatch = false
                    }
                  }
                })
              }
            }(this));
            getComponent = this.componentFinder(address.address_components);
            ref4 = getComponent('street_number', false), x = ref4[0], streetNum = ref4[1];
            ref5 = getComponent('route', false), streetAbbr = ref5[0], street = ref5[1];
            ref6 = getComponent('locality'), x = ref6[0], city = ref6[1];
            ref7 = getComponent('administrative_area_level_1'), stateAbbr = ref7[0], state = ref7[1];
            ref8 = getComponent('country'), countryAbbr = ref8[0], country = ref8[1];
            ref9 = getComponent('postal_code', false), postalCode = ref9[0], x = ref9[1];
            address = {
              streetNumber: streetNum,
              street: street,
              streetAbbr: streetAbbr,
              city: city,
              state: state,
              stateAbbr: stateAbbr,
              country: country,
              countryAbbr: countryAbbr,
              postalCode: postalCode,
              location: location
            }
          }
          _.each(address, function (_this) {
            return function (val, key) {
              return _this[key] = val
            }
          }(this))
        } else {
          this.addressStr = address
        }
      }
      Address.prototype.componentFinder = function (components) {
        return function (type, type2) {
          var it;
          if (type2 == null) {
            type2 = 'political'
          }
          it = _.find(components, function (c) {
            return c.types[0] === type && (!type2 || c.types[1] === type2)
          });
          return [
            it != null ? it.short_name : void 0,
            it != null ? it.long_name : void 0
          ]
        }
      };
      Address.prototype.toString = function (useCountryAbbr, useStateAbbr, useStreetAbbr) {
        var arr, countryVal, i, len, prop, ref, stateVal, str, streetVal;
        if (useCountryAbbr == null) {
          useCountryAbbr = true
        }
        if (useStateAbbr == null) {
          useStateAbbr = true
        }
        if (useStreetAbbr == null) {
          useStreetAbbr = false
        }
        if (!this.isObject) {
          return this.addressStr
        }
        arr = [];
        stateVal = useStateAbbr && this.generated ? 'stateAbbr' : 'state';
        countryVal = useCountryAbbr && this.generated ? 'countryAbbr' : 'country';
        streetVal = useStreetAbbr && this.generated ? 'streetAbbr' : 'street';
        ref = [
          streetVal,
          'city',
          stateVal,
          countryVal
        ];
        for (i = 0, len = ref.length; i < len; i++) {
          prop = ref[i];
          if (this[prop]) {
            arr.push(this[prop])
          }
        }
        str = arr.join(', ');
        if (this.streetNumber) {
          str = this.streetNumber + ' ' + str
        }
        return str
      };
      return Address
    }();
    /*
    validate an input address.

    inputAddr: validator.Address object or map with 'street', 'city', 'state', 'country' keys, or string address
    cb: function(err, validAddresses, inexactMatches, geocodingResponse)
        err - something went wrong calling the google api
        validAddresses - list of Address objects. These are exact matches to your input, and will have proper spelling, caps etc. Its best that you use this instead of what you had
        inexactMatches - list of Address objects. Incomplete addresses or addresses that do not match your input address. useful for 'did you mean?' type UIs
        geocodingResponse - the json object that i got from google API
 */
    defaultMatchType = matchType.streetAddress;
    exports.validate = function (inputAddr, addressType, cb) {
      var inputAddress, opts, protocol, qs;
      if (addressType == null) {
        addressType = defaultMatchType
      }
      if (arguments.length === 2) {
        cb = addressType;
        addressType = defaultMatchType
      }
      inputAddress = inputAddr instanceof Address ? inputAddr : new Address(inputAddr);
      qs = {
        'sensor': false,
        'address': inputAddress.toString(),
        region: options.countryBias,
        language: options.language
      };
      if (options.countryMatch) {
        qs.components = 'country:' + options.countryMatch
      }
      protocol = 'http';
      if (options.key) {
        qs.key = options.key;
        protocol = 'https'
      }
      opts = {
        json: true,
        url: protocol + '://maps.googleapis.com/maps/api/geocode/json',
        method: 'GET',
        qs: qs
      };
      return request(opts, function (err, response, body) {
        var inexactMatches, validAddresses;
        if (err) {
          return cb(err, null, null, body)
        }
        if (response.statusCode !== 200) {
          return cb(new Error('Google geocode API returned status code of ' + response.statusCode), [], [], body)
        }
        if (body.status.toLowerCase() !== 'ok') {
          return cb(new Error('Google returned error: ' + body.status + ' - ' + body.error_message), [], [], body)
        }
        if (body.results.length === 0) {
          return cb(null, [], [], body)
        }
        validAddresses = [];
        inexactMatches = [];
        _.each(body.results, function (result) {
          var address;
          address = new Address(result);
          if (addressType === matchType.unknown) {
            if (matchUnknownType(address, inputAddress)) {
              return validAddresses.push(address)
            } else {
              return inexactMatches.push(address)
            }
          } else if (addressType === address.matchType) {
            if (address.exactMatch) {
              return validAddresses.push(address)
            } else {
              return inexactMatches.push(address)
            }
          }
        });
        return cb(null, validAddresses, inexactMatches, body)
      })
    }
  });
  // source: /Users/dtai/work/verus/crowdstart/node_modules/address-validator/node_modules/underscore/underscore.js
  require.define('address-validator/node_modules/underscore/underscore', function (module, exports, __dirname, __filename) {
    //     Underscore.js 1.4.4
    //     http://underscorejs.org
    //     (c) 2009-2013 Jeremy Ashkenas, DocumentCloud Inc.
    //     Underscore may be freely distributed under the MIT license.
    (function () {
      // Baseline setup
      // --------------
      // Establish the root object, `window` in the browser, or `global` on the server.
      var root = this;
      // Save the previous value of the `_` variable.
      var previousUnderscore = root._;
      // Establish the object that gets returned to break out of a loop iteration.
      var breaker = {};
      // Save bytes in the minified (but not gzipped) version:
      var ArrayProto = Array.prototype, ObjProto = Object.prototype, FuncProto = Function.prototype;
      // Create quick reference variables for speed access to core prototypes.
      var push = ArrayProto.push, slice = ArrayProto.slice, concat = ArrayProto.concat, toString = ObjProto.toString, hasOwnProperty = ObjProto.hasOwnProperty;
      // All **ECMAScript 5** native function implementations that we hope to use
      // are declared here.
      var nativeForEach = ArrayProto.forEach, nativeMap = ArrayProto.map, nativeReduce = ArrayProto.reduce, nativeReduceRight = ArrayProto.reduceRight, nativeFilter = ArrayProto.filter, nativeEvery = ArrayProto.every, nativeSome = ArrayProto.some, nativeIndexOf = ArrayProto.indexOf, nativeLastIndexOf = ArrayProto.lastIndexOf, nativeIsArray = Array.isArray, nativeKeys = Object.keys, nativeBind = FuncProto.bind;
      // Create a safe reference to the Underscore object for use below.
      var _ = function (obj) {
        if (obj instanceof _)
          return obj;
        if (!(this instanceof _))
          return new _(obj);
        this._wrapped = obj
      };
      // Export the Underscore object for **Node.js**, with
      // backwards-compatibility for the old `require()` API. If we're in
      // the browser, add `_` as a global object via a string identifier,
      // for Closure Compiler "advanced" mode.
      if (typeof exports !== 'undefined') {
        if (typeof module !== 'undefined' && module.exports) {
          exports = module.exports = _
        }
        exports._ = _
      } else {
        root._ = _
      }
      // Current version.
      _.VERSION = '1.4.4';
      // Collection Functions
      // --------------------
      // The cornerstone, an `each` implementation, aka `forEach`.
      // Handles objects with the built-in `forEach`, arrays, and raw objects.
      // Delegates to **ECMAScript 5**'s native `forEach` if available.
      var each = _.each = _.forEach = function (obj, iterator, context) {
        if (obj == null)
          return;
        if (nativeForEach && obj.forEach === nativeForEach) {
          obj.forEach(iterator, context)
        } else if (obj.length === +obj.length) {
          for (var i = 0, l = obj.length; i < l; i++) {
            if (iterator.call(context, obj[i], i, obj) === breaker)
              return
          }
        } else {
          for (var key in obj) {
            if (_.has(obj, key)) {
              if (iterator.call(context, obj[key], key, obj) === breaker)
                return
            }
          }
        }
      };
      // Return the results of applying the iterator to each element.
      // Delegates to **ECMAScript 5**'s native `map` if available.
      _.map = _.collect = function (obj, iterator, context) {
        var results = [];
        if (obj == null)
          return results;
        if (nativeMap && obj.map === nativeMap)
          return obj.map(iterator, context);
        each(obj, function (value, index, list) {
          results[results.length] = iterator.call(context, value, index, list)
        });
        return results
      };
      var reduceError = 'Reduce of empty array with no initial value';
      // **Reduce** builds up a single result from a list of values, aka `inject`,
      // or `foldl`. Delegates to **ECMAScript 5**'s native `reduce` if available.
      _.reduce = _.foldl = _.inject = function (obj, iterator, memo, context) {
        var initial = arguments.length > 2;
        if (obj == null)
          obj = [];
        if (nativeReduce && obj.reduce === nativeReduce) {
          if (context)
            iterator = _.bind(iterator, context);
          return initial ? obj.reduce(iterator, memo) : obj.reduce(iterator)
        }
        each(obj, function (value, index, list) {
          if (!initial) {
            memo = value;
            initial = true
          } else {
            memo = iterator.call(context, memo, value, index, list)
          }
        });
        if (!initial)
          throw new TypeError(reduceError);
        return memo
      };
      // The right-associative version of reduce, also known as `foldr`.
      // Delegates to **ECMAScript 5**'s native `reduceRight` if available.
      _.reduceRight = _.foldr = function (obj, iterator, memo, context) {
        var initial = arguments.length > 2;
        if (obj == null)
          obj = [];
        if (nativeReduceRight && obj.reduceRight === nativeReduceRight) {
          if (context)
            iterator = _.bind(iterator, context);
          return initial ? obj.reduceRight(iterator, memo) : obj.reduceRight(iterator)
        }
        var length = obj.length;
        if (length !== +length) {
          var keys = _.keys(obj);
          length = keys.length
        }
        each(obj, function (value, index, list) {
          index = keys ? keys[--length] : --length;
          if (!initial) {
            memo = obj[index];
            initial = true
          } else {
            memo = iterator.call(context, memo, obj[index], index, list)
          }
        });
        if (!initial)
          throw new TypeError(reduceError);
        return memo
      };
      // Return the first value which passes a truth test. Aliased as `detect`.
      _.find = _.detect = function (obj, iterator, context) {
        var result;
        any(obj, function (value, index, list) {
          if (iterator.call(context, value, index, list)) {
            result = value;
            return true
          }
        });
        return result
      };
      // Return all the elements that pass a truth test.
      // Delegates to **ECMAScript 5**'s native `filter` if available.
      // Aliased as `select`.
      _.filter = _.select = function (obj, iterator, context) {
        var results = [];
        if (obj == null)
          return results;
        if (nativeFilter && obj.filter === nativeFilter)
          return obj.filter(iterator, context);
        each(obj, function (value, index, list) {
          if (iterator.call(context, value, index, list))
            results[results.length] = value
        });
        return results
      };
      // Return all the elements for which a truth test fails.
      _.reject = function (obj, iterator, context) {
        return _.filter(obj, function (value, index, list) {
          return !iterator.call(context, value, index, list)
        }, context)
      };
      // Determine whether all of the elements match a truth test.
      // Delegates to **ECMAScript 5**'s native `every` if available.
      // Aliased as `all`.
      _.every = _.all = function (obj, iterator, context) {
        iterator || (iterator = _.identity);
        var result = true;
        if (obj == null)
          return result;
        if (nativeEvery && obj.every === nativeEvery)
          return obj.every(iterator, context);
        each(obj, function (value, index, list) {
          if (!(result = result && iterator.call(context, value, index, list)))
            return breaker
        });
        return !!result
      };
      // Determine if at least one element in the object matches a truth test.
      // Delegates to **ECMAScript 5**'s native `some` if available.
      // Aliased as `any`.
      var any = _.some = _.any = function (obj, iterator, context) {
        iterator || (iterator = _.identity);
        var result = false;
        if (obj == null)
          return result;
        if (nativeSome && obj.some === nativeSome)
          return obj.some(iterator, context);
        each(obj, function (value, index, list) {
          if (result || (result = iterator.call(context, value, index, list)))
            return breaker
        });
        return !!result
      };
      // Determine if the array or object contains a given value (using `===`).
      // Aliased as `include`.
      _.contains = _.include = function (obj, target) {
        if (obj == null)
          return false;
        if (nativeIndexOf && obj.indexOf === nativeIndexOf)
          return obj.indexOf(target) != -1;
        return any(obj, function (value) {
          return value === target
        })
      };
      // Invoke a method (with arguments) on every item in a collection.
      _.invoke = function (obj, method) {
        var args = slice.call(arguments, 2);
        var isFunc = _.isFunction(method);
        return _.map(obj, function (value) {
          return (isFunc ? method : value[method]).apply(value, args)
        })
      };
      // Convenience version of a common use case of `map`: fetching a property.
      _.pluck = function (obj, key) {
        return _.map(obj, function (value) {
          return value[key]
        })
      };
      // Convenience version of a common use case of `filter`: selecting only objects
      // containing specific `key:value` pairs.
      _.where = function (obj, attrs, first) {
        if (_.isEmpty(attrs))
          return first ? null : [];
        return _[first ? 'find' : 'filter'](obj, function (value) {
          for (var key in attrs) {
            if (attrs[key] !== value[key])
              return false
          }
          return true
        })
      };
      // Convenience version of a common use case of `find`: getting the first object
      // containing specific `key:value` pairs.
      _.findWhere = function (obj, attrs) {
        return _.where(obj, attrs, true)
      };
      // Return the maximum element or (element-based computation).
      // Can't optimize arrays of integers longer than 65,535 elements.
      // See: https://bugs.webkit.org/show_bug.cgi?id=80797
      _.max = function (obj, iterator, context) {
        if (!iterator && _.isArray(obj) && obj[0] === +obj[0] && obj.length < 65535) {
          return Math.max.apply(Math, obj)
        }
        if (!iterator && _.isEmpty(obj))
          return -Infinity;
        var result = {
          computed: -Infinity,
          value: -Infinity
        };
        each(obj, function (value, index, list) {
          var computed = iterator ? iterator.call(context, value, index, list) : value;
          computed >= result.computed && (result = {
            value: value,
            computed: computed
          })
        });
        return result.value
      };
      // Return the minimum element (or element-based computation).
      _.min = function (obj, iterator, context) {
        if (!iterator && _.isArray(obj) && obj[0] === +obj[0] && obj.length < 65535) {
          return Math.min.apply(Math, obj)
        }
        if (!iterator && _.isEmpty(obj))
          return Infinity;
        var result = {
          computed: Infinity,
          value: Infinity
        };
        each(obj, function (value, index, list) {
          var computed = iterator ? iterator.call(context, value, index, list) : value;
          computed < result.computed && (result = {
            value: value,
            computed: computed
          })
        });
        return result.value
      };
      // Shuffle an array.
      _.shuffle = function (obj) {
        var rand;
        var index = 0;
        var shuffled = [];
        each(obj, function (value) {
          rand = _.random(index++);
          shuffled[index - 1] = shuffled[rand];
          shuffled[rand] = value
        });
        return shuffled
      };
      // An internal function to generate lookup iterators.
      var lookupIterator = function (value) {
        return _.isFunction(value) ? value : function (obj) {
          return obj[value]
        }
      };
      // Sort the object's values by a criterion produced by an iterator.
      _.sortBy = function (obj, value, context) {
        var iterator = lookupIterator(value);
        return _.pluck(_.map(obj, function (value, index, list) {
          return {
            value: value,
            index: index,
            criteria: iterator.call(context, value, index, list)
          }
        }).sort(function (left, right) {
          var a = left.criteria;
          var b = right.criteria;
          if (a !== b) {
            if (a > b || a === void 0)
              return 1;
            if (a < b || b === void 0)
              return -1
          }
          return left.index < right.index ? -1 : 1
        }), 'value')
      };
      // An internal function used for aggregate "group by" operations.
      var group = function (obj, value, context, behavior) {
        var result = {};
        var iterator = lookupIterator(value || _.identity);
        each(obj, function (value, index) {
          var key = iterator.call(context, value, index, obj);
          behavior(result, key, value)
        });
        return result
      };
      // Groups the object's values by a criterion. Pass either a string attribute
      // to group by, or a function that returns the criterion.
      _.groupBy = function (obj, value, context) {
        return group(obj, value, context, function (result, key, value) {
          (_.has(result, key) ? result[key] : result[key] = []).push(value)
        })
      };
      // Counts instances of an object that group by a certain criterion. Pass
      // either a string attribute to count by, or a function that returns the
      // criterion.
      _.countBy = function (obj, value, context) {
        return group(obj, value, context, function (result, key) {
          if (!_.has(result, key))
            result[key] = 0;
          result[key]++
        })
      };
      // Use a comparator function to figure out the smallest index at which
      // an object should be inserted so as to maintain order. Uses binary search.
      _.sortedIndex = function (array, obj, iterator, context) {
        iterator = iterator == null ? _.identity : lookupIterator(iterator);
        var value = iterator.call(context, obj);
        var low = 0, high = array.length;
        while (low < high) {
          var mid = low + high >>> 1;
          iterator.call(context, array[mid]) < value ? low = mid + 1 : high = mid
        }
        return low
      };
      // Safely convert anything iterable into a real, live array.
      _.toArray = function (obj) {
        if (!obj)
          return [];
        if (_.isArray(obj))
          return slice.call(obj);
        if (obj.length === +obj.length)
          return _.map(obj, _.identity);
        return _.values(obj)
      };
      // Return the number of elements in an object.
      _.size = function (obj) {
        if (obj == null)
          return 0;
        return obj.length === +obj.length ? obj.length : _.keys(obj).length
      };
      // Array Functions
      // ---------------
      // Get the first element of an array. Passing **n** will return the first N
      // values in the array. Aliased as `head` and `take`. The **guard** check
      // allows it to work with `_.map`.
      _.first = _.head = _.take = function (array, n, guard) {
        if (array == null)
          return void 0;
        return n != null && !guard ? slice.call(array, 0, n) : array[0]
      };
      // Returns everything but the last entry of the array. Especially useful on
      // the arguments object. Passing **n** will return all the values in
      // the array, excluding the last N. The **guard** check allows it to work with
      // `_.map`.
      _.initial = function (array, n, guard) {
        return slice.call(array, 0, array.length - (n == null || guard ? 1 : n))
      };
      // Get the last element of an array. Passing **n** will return the last N
      // values in the array. The **guard** check allows it to work with `_.map`.
      _.last = function (array, n, guard) {
        if (array == null)
          return void 0;
        if (n != null && !guard) {
          return slice.call(array, Math.max(array.length - n, 0))
        } else {
          return array[array.length - 1]
        }
      };
      // Returns everything but the first entry of the array. Aliased as `tail` and `drop`.
      // Especially useful on the arguments object. Passing an **n** will return
      // the rest N values in the array. The **guard**
      // check allows it to work with `_.map`.
      _.rest = _.tail = _.drop = function (array, n, guard) {
        return slice.call(array, n == null || guard ? 1 : n)
      };
      // Trim out all falsy values from an array.
      _.compact = function (array) {
        return _.filter(array, _.identity)
      };
      // Internal implementation of a recursive `flatten` function.
      var flatten = function (input, shallow, output) {
        each(input, function (value) {
          if (_.isArray(value)) {
            shallow ? push.apply(output, value) : flatten(value, shallow, output)
          } else {
            output.push(value)
          }
        });
        return output
      };
      // Return a completely flattened version of an array.
      _.flatten = function (array, shallow) {
        return flatten(array, shallow, [])
      };
      // Return a version of the array that does not contain the specified value(s).
      _.without = function (array) {
        return _.difference(array, slice.call(arguments, 1))
      };
      // Produce a duplicate-free version of the array. If the array has already
      // been sorted, you have the option of using a faster algorithm.
      // Aliased as `unique`.
      _.uniq = _.unique = function (array, isSorted, iterator, context) {
        if (_.isFunction(isSorted)) {
          context = iterator;
          iterator = isSorted;
          isSorted = false
        }
        var initial = iterator ? _.map(array, iterator, context) : array;
        var results = [];
        var seen = [];
        each(initial, function (value, index) {
          if (isSorted ? !index || seen[seen.length - 1] !== value : !_.contains(seen, value)) {
            seen.push(value);
            results.push(array[index])
          }
        });
        return results
      };
      // Produce an array that contains the union: each distinct element from all of
      // the passed-in arrays.
      _.union = function () {
        return _.uniq(concat.apply(ArrayProto, arguments))
      };
      // Produce an array that contains every item shared between all the
      // passed-in arrays.
      _.intersection = function (array) {
        var rest = slice.call(arguments, 1);
        return _.filter(_.uniq(array), function (item) {
          return _.every(rest, function (other) {
            return _.indexOf(other, item) >= 0
          })
        })
      };
      // Take the difference between one array and a number of other arrays.
      // Only the elements present in just the first array will remain.
      _.difference = function (array) {
        var rest = concat.apply(ArrayProto, slice.call(arguments, 1));
        return _.filter(array, function (value) {
          return !_.contains(rest, value)
        })
      };
      // Zip together multiple lists into a single array -- elements that share
      // an index go together.
      _.zip = function () {
        var args = slice.call(arguments);
        var length = _.max(_.pluck(args, 'length'));
        var results = new Array(length);
        for (var i = 0; i < length; i++) {
          results[i] = _.pluck(args, '' + i)
        }
        return results
      };
      // Converts lists into objects. Pass either a single array of `[key, value]`
      // pairs, or two parallel arrays of the same length -- one of keys, and one of
      // the corresponding values.
      _.object = function (list, values) {
        if (list == null)
          return {};
        var result = {};
        for (var i = 0, l = list.length; i < l; i++) {
          if (values) {
            result[list[i]] = values[i]
          } else {
            result[list[i][0]] = list[i][1]
          }
        }
        return result
      };
      // If the browser doesn't supply us with indexOf (I'm looking at you, **MSIE**),
      // we need this function. Return the position of the first occurrence of an
      // item in an array, or -1 if the item is not included in the array.
      // Delegates to **ECMAScript 5**'s native `indexOf` if available.
      // If the array is large and already in sort order, pass `true`
      // for **isSorted** to use binary search.
      _.indexOf = function (array, item, isSorted) {
        if (array == null)
          return -1;
        var i = 0, l = array.length;
        if (isSorted) {
          if (typeof isSorted == 'number') {
            i = isSorted < 0 ? Math.max(0, l + isSorted) : isSorted
          } else {
            i = _.sortedIndex(array, item);
            return array[i] === item ? i : -1
          }
        }
        if (nativeIndexOf && array.indexOf === nativeIndexOf)
          return array.indexOf(item, isSorted);
        for (; i < l; i++)
          if (array[i] === item)
            return i;
        return -1
      };
      // Delegates to **ECMAScript 5**'s native `lastIndexOf` if available.
      _.lastIndexOf = function (array, item, from) {
        if (array == null)
          return -1;
        var hasIndex = from != null;
        if (nativeLastIndexOf && array.lastIndexOf === nativeLastIndexOf) {
          return hasIndex ? array.lastIndexOf(item, from) : array.lastIndexOf(item)
        }
        var i = hasIndex ? from : array.length;
        while (i--)
          if (array[i] === item)
            return i;
        return -1
      };
      // Generate an integer Array containing an arithmetic progression. A port of
      // the native Python `range()` function. See
      // [the Python documentation](http://docs.python.org/library/functions.html#range).
      _.range = function (start, stop, step) {
        if (arguments.length <= 1) {
          stop = start || 0;
          start = 0
        }
        step = arguments[2] || 1;
        var len = Math.max(Math.ceil((stop - start) / step), 0);
        var idx = 0;
        var range = new Array(len);
        while (idx < len) {
          range[idx++] = start;
          start += step
        }
        return range
      };
      // Function (ahem) Functions
      // ------------------
      // Create a function bound to a given object (assigning `this`, and arguments,
      // optionally). Delegates to **ECMAScript 5**'s native `Function.bind` if
      // available.
      _.bind = function (func, context) {
        if (func.bind === nativeBind && nativeBind)
          return nativeBind.apply(func, slice.call(arguments, 1));
        var args = slice.call(arguments, 2);
        return function () {
          return func.apply(context, args.concat(slice.call(arguments)))
        }
      };
      // Partially apply a function by creating a version that has had some of its
      // arguments pre-filled, without changing its dynamic `this` context.
      _.partial = function (func) {
        var args = slice.call(arguments, 1);
        return function () {
          return func.apply(this, args.concat(slice.call(arguments)))
        }
      };
      // Bind all of an object's methods to that object. Useful for ensuring that
      // all callbacks defined on an object belong to it.
      _.bindAll = function (obj) {
        var funcs = slice.call(arguments, 1);
        if (funcs.length === 0)
          funcs = _.functions(obj);
        each(funcs, function (f) {
          obj[f] = _.bind(obj[f], obj)
        });
        return obj
      };
      // Memoize an expensive function by storing its results.
      _.memoize = function (func, hasher) {
        var memo = {};
        hasher || (hasher = _.identity);
        return function () {
          var key = hasher.apply(this, arguments);
          return _.has(memo, key) ? memo[key] : memo[key] = func.apply(this, arguments)
        }
      };
      // Delays a function for the given number of milliseconds, and then calls
      // it with the arguments supplied.
      _.delay = function (func, wait) {
        var args = slice.call(arguments, 2);
        return setTimeout(function () {
          return func.apply(null, args)
        }, wait)
      };
      // Defers a function, scheduling it to run after the current call stack has
      // cleared.
      _.defer = function (func) {
        return _.delay.apply(_, [
          func,
          1
        ].concat(slice.call(arguments, 1)))
      };
      // Returns a function, that, when invoked, will only be triggered at most once
      // during a given window of time.
      _.throttle = function (func, wait) {
        var context, args, timeout, result;
        var previous = 0;
        var later = function () {
          previous = new Date;
          timeout = null;
          result = func.apply(context, args)
        };
        return function () {
          var now = new Date;
          var remaining = wait - (now - previous);
          context = this;
          args = arguments;
          if (remaining <= 0) {
            clearTimeout(timeout);
            timeout = null;
            previous = now;
            result = func.apply(context, args)
          } else if (!timeout) {
            timeout = setTimeout(later, remaining)
          }
          return result
        }
      };
      // Returns a function, that, as long as it continues to be invoked, will not
      // be triggered. The function will be called after it stops being called for
      // N milliseconds. If `immediate` is passed, trigger the function on the
      // leading edge, instead of the trailing.
      _.debounce = function (func, wait, immediate) {
        var timeout, result;
        return function () {
          var context = this, args = arguments;
          var later = function () {
            timeout = null;
            if (!immediate)
              result = func.apply(context, args)
          };
          var callNow = immediate && !timeout;
          clearTimeout(timeout);
          timeout = setTimeout(later, wait);
          if (callNow)
            result = func.apply(context, args);
          return result
        }
      };
      // Returns a function that will be executed at most one time, no matter how
      // often you call it. Useful for lazy initialization.
      _.once = function (func) {
        var ran = false, memo;
        return function () {
          if (ran)
            return memo;
          ran = true;
          memo = func.apply(this, arguments);
          func = null;
          return memo
        }
      };
      // Returns the first function passed as an argument to the second,
      // allowing you to adjust arguments, run code before and after, and
      // conditionally execute the original function.
      _.wrap = function (func, wrapper) {
        return function () {
          var args = [func];
          push.apply(args, arguments);
          return wrapper.apply(this, args)
        }
      };
      // Returns a function that is the composition of a list of functions, each
      // consuming the return value of the function that follows.
      _.compose = function () {
        var funcs = arguments;
        return function () {
          var args = arguments;
          for (var i = funcs.length - 1; i >= 0; i--) {
            args = [funcs[i].apply(this, args)]
          }
          return args[0]
        }
      };
      // Returns a function that will only be executed after being called N times.
      _.after = function (times, func) {
        if (times <= 0)
          return func();
        return function () {
          if (--times < 1) {
            return func.apply(this, arguments)
          }
        }
      };
      // Object Functions
      // ----------------
      // Retrieve the names of an object's properties.
      // Delegates to **ECMAScript 5**'s native `Object.keys`
      _.keys = nativeKeys || function (obj) {
        if (obj !== Object(obj))
          throw new TypeError('Invalid object');
        var keys = [];
        for (var key in obj)
          if (_.has(obj, key))
            keys[keys.length] = key;
        return keys
      };
      // Retrieve the values of an object's properties.
      _.values = function (obj) {
        var values = [];
        for (var key in obj)
          if (_.has(obj, key))
            values.push(obj[key]);
        return values
      };
      // Convert an object into a list of `[key, value]` pairs.
      _.pairs = function (obj) {
        var pairs = [];
        for (var key in obj)
          if (_.has(obj, key))
            pairs.push([
              key,
              obj[key]
            ]);
        return pairs
      };
      // Invert the keys and values of an object. The values must be serializable.
      _.invert = function (obj) {
        var result = {};
        for (var key in obj)
          if (_.has(obj, key))
            result[obj[key]] = key;
        return result
      };
      // Return a sorted list of the function names available on the object.
      // Aliased as `methods`
      _.functions = _.methods = function (obj) {
        var names = [];
        for (var key in obj) {
          if (_.isFunction(obj[key]))
            names.push(key)
        }
        return names.sort()
      };
      // Extend a given object with all the properties in passed-in object(s).
      _.extend = function (obj) {
        each(slice.call(arguments, 1), function (source) {
          if (source) {
            for (var prop in source) {
              obj[prop] = source[prop]
            }
          }
        });
        return obj
      };
      // Return a copy of the object only containing the whitelisted properties.
      _.pick = function (obj) {
        var copy = {};
        var keys = concat.apply(ArrayProto, slice.call(arguments, 1));
        each(keys, function (key) {
          if (key in obj)
            copy[key] = obj[key]
        });
        return copy
      };
      // Return a copy of the object without the blacklisted properties.
      _.omit = function (obj) {
        var copy = {};
        var keys = concat.apply(ArrayProto, slice.call(arguments, 1));
        for (var key in obj) {
          if (!_.contains(keys, key))
            copy[key] = obj[key]
        }
        return copy
      };
      // Fill in a given object with default properties.
      _.defaults = function (obj) {
        each(slice.call(arguments, 1), function (source) {
          if (source) {
            for (var prop in source) {
              if (obj[prop] == null)
                obj[prop] = source[prop]
            }
          }
        });
        return obj
      };
      // Create a (shallow-cloned) duplicate of an object.
      _.clone = function (obj) {
        if (!_.isObject(obj))
          return obj;
        return _.isArray(obj) ? obj.slice() : _.extend({}, obj)
      };
      // Invokes interceptor with the obj, and then returns obj.
      // The primary purpose of this method is to "tap into" a method chain, in
      // order to perform operations on intermediate results within the chain.
      _.tap = function (obj, interceptor) {
        interceptor(obj);
        return obj
      };
      // Internal recursive comparison function for `isEqual`.
      var eq = function (a, b, aStack, bStack) {
        // Identical objects are equal. `0 === -0`, but they aren't identical.
        // See the Harmony `egal` proposal: http://wiki.ecmascript.org/doku.php?id=harmony:egal.
        if (a === b)
          return a !== 0 || 1 / a == 1 / b;
        // A strict comparison is necessary because `null == undefined`.
        if (a == null || b == null)
          return a === b;
        // Unwrap any wrapped objects.
        if (a instanceof _)
          a = a._wrapped;
        if (b instanceof _)
          b = b._wrapped;
        // Compare `[[Class]]` names.
        var className = toString.call(a);
        if (className != toString.call(b))
          return false;
        switch (className) {
        // Strings, numbers, dates, and booleans are compared by value.
        case '[object String]':
          // Primitives and their corresponding object wrappers are equivalent; thus, `"5"` is
          // equivalent to `new String("5")`.
          return a == String(b);
        case '[object Number]':
          // `NaN`s are equivalent, but non-reflexive. An `egal` comparison is performed for
          // other numeric values.
          return a != +a ? b != +b : a == 0 ? 1 / a == 1 / b : a == +b;
        case '[object Date]':
        case '[object Boolean]':
          // Coerce dates and booleans to numeric primitive values. Dates are compared by their
          // millisecond representations. Note that invalid dates with millisecond representations
          // of `NaN` are not equivalent.
          return +a == +b;
        // RegExps are compared by their source patterns and flags.
        case '[object RegExp]':
          return a.source == b.source && a.global == b.global && a.multiline == b.multiline && a.ignoreCase == b.ignoreCase
        }
        if (typeof a != 'object' || typeof b != 'object')
          return false;
        // Assume equality for cyclic structures. The algorithm for detecting cyclic
        // structures is adapted from ES 5.1 section 15.12.3, abstract operation `JO`.
        var length = aStack.length;
        while (length--) {
          // Linear search. Performance is inversely proportional to the number of
          // unique nested structures.
          if (aStack[length] == a)
            return bStack[length] == b
        }
        // Add the first object to the stack of traversed objects.
        aStack.push(a);
        bStack.push(b);
        var size = 0, result = true;
        // Recursively compare objects and arrays.
        if (className == '[object Array]') {
          // Compare array lengths to determine if a deep comparison is necessary.
          size = a.length;
          result = size == b.length;
          if (result) {
            // Deep compare the contents, ignoring non-numeric properties.
            while (size--) {
              if (!(result = eq(a[size], b[size], aStack, bStack)))
                break
            }
          }
        } else {
          // Objects with different constructors are not equivalent, but `Object`s
          // from different frames are.
          var aCtor = a.constructor, bCtor = b.constructor;
          if (aCtor !== bCtor && !(_.isFunction(aCtor) && aCtor instanceof aCtor && _.isFunction(bCtor) && bCtor instanceof bCtor)) {
            return false
          }
          // Deep compare objects.
          for (var key in a) {
            if (_.has(a, key)) {
              // Count the expected number of properties.
              size++;
              // Deep compare each member.
              if (!(result = _.has(b, key) && eq(a[key], b[key], aStack, bStack)))
                break
            }
          }
          // Ensure that both objects contain the same number of properties.
          if (result) {
            for (key in b) {
              if (_.has(b, key) && !size--)
                break
            }
            result = !size
          }
        }
        // Remove the first object from the stack of traversed objects.
        aStack.pop();
        bStack.pop();
        return result
      };
      // Perform a deep comparison to check if two objects are equal.
      _.isEqual = function (a, b) {
        return eq(a, b, [], [])
      };
      // Is a given array, string, or object empty?
      // An "empty" object has no enumerable own-properties.
      _.isEmpty = function (obj) {
        if (obj == null)
          return true;
        if (_.isArray(obj) || _.isString(obj))
          return obj.length === 0;
        for (var key in obj)
          if (_.has(obj, key))
            return false;
        return true
      };
      // Is a given value a DOM element?
      _.isElement = function (obj) {
        return !!(obj && obj.nodeType === 1)
      };
      // Is a given value an array?
      // Delegates to ECMA5's native Array.isArray
      _.isArray = nativeIsArray || function (obj) {
        return toString.call(obj) == '[object Array]'
      };
      // Is a given variable an object?
      _.isObject = function (obj) {
        return obj === Object(obj)
      };
      // Add some isType methods: isArguments, isFunction, isString, isNumber, isDate, isRegExp.
      each([
        'Arguments',
        'Function',
        'String',
        'Number',
        'Date',
        'RegExp'
      ], function (name) {
        _['is' + name] = function (obj) {
          return toString.call(obj) == '[object ' + name + ']'
        }
      });
      // Define a fallback version of the method in browsers (ahem, IE), where
      // there isn't any inspectable "Arguments" type.
      if (!_.isArguments(arguments)) {
        _.isArguments = function (obj) {
          return !!(obj && _.has(obj, 'callee'))
        }
      }
      // Optimize `isFunction` if appropriate.
      if (typeof /./ !== 'function') {
        _.isFunction = function (obj) {
          return typeof obj === 'function'
        }
      }
      // Is a given object a finite number?
      _.isFinite = function (obj) {
        return isFinite(obj) && !isNaN(parseFloat(obj))
      };
      // Is the given value `NaN`? (NaN is the only number which does not equal itself).
      _.isNaN = function (obj) {
        return _.isNumber(obj) && obj != +obj
      };
      // Is a given value a boolean?
      _.isBoolean = function (obj) {
        return obj === true || obj === false || toString.call(obj) == '[object Boolean]'
      };
      // Is a given value equal to null?
      _.isNull = function (obj) {
        return obj === null
      };
      // Is a given variable undefined?
      _.isUndefined = function (obj) {
        return obj === void 0
      };
      // Shortcut function for checking if an object has a given property directly
      // on itself (in other words, not on a prototype).
      _.has = function (obj, key) {
        return hasOwnProperty.call(obj, key)
      };
      // Utility Functions
      // -----------------
      // Run Underscore.js in *noConflict* mode, returning the `_` variable to its
      // previous owner. Returns a reference to the Underscore object.
      _.noConflict = function () {
        root._ = previousUnderscore;
        return this
      };
      // Keep the identity function around for default iterators.
      _.identity = function (value) {
        return value
      };
      // Run a function **n** times.
      _.times = function (n, iterator, context) {
        var accum = Array(n);
        for (var i = 0; i < n; i++)
          accum[i] = iterator.call(context, i);
        return accum
      };
      // Return a random integer between min and max (inclusive).
      _.random = function (min, max) {
        if (max == null) {
          max = min;
          min = 0
        }
        return min + Math.floor(Math.random() * (max - min + 1))
      };
      // List of HTML entities for escaping.
      var entityMap = {
        escape: {
          '&': '&amp;',
          '<': '&lt;',
          '>': '&gt;',
          '"': '&quot;',
          "'": '&#x27;',
          '/': '&#x2F;'
        }
      };
      entityMap.unescape = _.invert(entityMap.escape);
      // Regexes containing the keys and values listed immediately above.
      var entityRegexes = {
        escape: new RegExp('[' + _.keys(entityMap.escape).join('') + ']', 'g'),
        unescape: new RegExp('(' + _.keys(entityMap.unescape).join('|') + ')', 'g')
      };
      // Functions for escaping and unescaping strings to/from HTML interpolation.
      _.each([
        'escape',
        'unescape'
      ], function (method) {
        _[method] = function (string) {
          if (string == null)
            return '';
          return ('' + string).replace(entityRegexes[method], function (match) {
            return entityMap[method][match]
          })
        }
      });
      // If the value of the named property is a function then invoke it;
      // otherwise, return it.
      _.result = function (object, property) {
        if (object == null)
          return null;
        var value = object[property];
        return _.isFunction(value) ? value.call(object) : value
      };
      // Add your own custom functions to the Underscore object.
      _.mixin = function (obj) {
        each(_.functions(obj), function (name) {
          var func = _[name] = obj[name];
          _.prototype[name] = function () {
            var args = [this._wrapped];
            push.apply(args, arguments);
            return result.call(this, func.apply(_, args))
          }
        })
      };
      // Generate a unique integer id (unique within the entire client session).
      // Useful for temporary DOM ids.
      var idCounter = 0;
      _.uniqueId = function (prefix) {
        var id = ++idCounter + '';
        return prefix ? prefix + id : id
      };
      // By default, Underscore uses ERB-style template delimiters, change the
      // following template settings to use alternative delimiters.
      _.templateSettings = {
        evaluate: /<%([\s\S]+?)%>/g,
        interpolate: /<%=([\s\S]+?)%>/g,
        escape: /<%-([\s\S]+?)%>/g
      };
      // When customizing `templateSettings`, if you don't want to define an
      // interpolation, evaluation or escaping regex, we need one that is
      // guaranteed not to match.
      var noMatch = /(.)^/;
      // Certain characters need to be escaped so that they can be put into a
      // string literal.
      var escapes = {
        "'": "'",
        '\\': '\\',
        '\r': 'r',
        '\n': 'n',
        '	': 't',
        '\u2028': 'u2028',
        '\u2029': 'u2029'
      };
      var escaper = /\\|'|\r|\n|\t|\u2028|\u2029/g;
      // JavaScript micro-templating, similar to John Resig's implementation.
      // Underscore templating handles arbitrary delimiters, preserves whitespace,
      // and correctly escapes quotes within interpolated code.
      _.template = function (text, data, settings) {
        var render;
        settings = _.defaults({}, settings, _.templateSettings);
        // Combine delimiters into one regular expression via alternation.
        var matcher = new RegExp([
          (settings.escape || noMatch).source,
          (settings.interpolate || noMatch).source,
          (settings.evaluate || noMatch).source
        ].join('|') + '|$', 'g');
        // Compile the template source, escaping string literals appropriately.
        var index = 0;
        var source = "__p+='";
        text.replace(matcher, function (match, escape, interpolate, evaluate, offset) {
          source += text.slice(index, offset).replace(escaper, function (match) {
            return '\\' + escapes[match]
          });
          if (escape) {
            source += "'+\n((__t=(" + escape + "))==null?'':_.escape(__t))+\n'"
          }
          if (interpolate) {
            source += "'+\n((__t=(" + interpolate + "))==null?'':__t)+\n'"
          }
          if (evaluate) {
            source += "';\n" + evaluate + "\n__p+='"
          }
          index = offset + match.length;
          return match
        });
        source += "';\n";
        // If a variable is not specified, place data values in local scope.
        if (!settings.variable)
          source = 'with(obj||{}){\n' + source + '}\n';
        source = "var __t,__p='',__j=Array.prototype.join," + "print=function(){__p+=__j.call(arguments,'');};\n" + source + 'return __p;\n';
        try {
          render = new Function(settings.variable || 'obj', '_', source)
        } catch (e) {
          e.source = source;
          throw e
        }
        if (data)
          return render(data, _);
        var template = function (data) {
          return render.call(this, data, _)
        };
        // Provide the compiled function source as a convenience for precompilation.
        template.source = 'function(' + (settings.variable || 'obj') + '){\n' + source + '}';
        return template
      };
      // Add a "chain" function, which will delegate to the wrapper.
      _.chain = function (obj) {
        return _(obj).chain()
      };
      // OOP
      // ---------------
      // If Underscore is called as a function, it returns a wrapped object that
      // can be used OO-style. This wrapper holds altered versions of all the
      // underscore functions. Wrapped objects may be chained.
      // Helper function to continue chaining intermediate results.
      var result = function (obj) {
        return this._chain ? _(obj).chain() : obj
      };
      // Add all of the Underscore functions to the wrapper object.
      _.mixin(_);
      // Add all mutator Array functions to the wrapper.
      each([
        'pop',
        'push',
        'reverse',
        'shift',
        'sort',
        'splice',
        'unshift'
      ], function (name) {
        var method = ArrayProto[name];
        _.prototype[name] = function () {
          var obj = this._wrapped;
          method.apply(obj, arguments);
          if ((name == 'shift' || name == 'splice') && obj.length === 0)
            delete obj[0];
          return result.call(this, obj)
        }
      });
      // Add all accessor Array functions to the wrapper.
      each([
        'concat',
        'join',
        'slice'
      ], function (name) {
        var method = ArrayProto[name];
        _.prototype[name] = function () {
          return result.call(this, method.apply(this._wrapped, arguments))
        }
      });
      _.extend(_.prototype, {
        // Start chaining a wrapped Underscore object.
        chain: function () {
          this._chain = true;
          return this
        },
        // Extracts the result from a wrapped and chained object.
        value: function () {
          return this._wrapped
        }
      })
    }.call(this))
  });
  // source: /Users/dtai/work/verus/crowdstart/node_modules/requisite/node_modules/browser-request/index.js
  require.define('requisite/node_modules/browser-request', function (module, exports, __dirname, __filename) {
    // Browser Request
    //
    // Licensed under the Apache License, Version 2.0 (the "License");
    // you may not use this file except in compliance with the License.
    // You may obtain a copy of the License at
    //
    //     http://www.apache.org/licenses/LICENSE-2.0
    //
    // Unless required by applicable law or agreed to in writing, software
    // distributed under the License is distributed on an "AS IS" BASIS,
    // WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    // See the License for the specific language governing permissions and
    // limitations under the License.
    // UMD HEADER START 
    (function (root, factory) {
      if (typeof define === 'function' && define.amd) {
        // AMD. Register as an anonymous module.
        define([], factory)
      } else if (typeof exports === 'object') {
        // Node. Does not work with strict CommonJS, but
        // only CommonJS-like enviroments that support module.exports,
        // like Node.
        module.exports = factory()
      } else {
        // Browser globals (root is window)
        root.returnExports = factory()
      }
    }(this, function () {
      // UMD HEADER END
      var XHR = XMLHttpRequest;
      if (!XHR)
        throw new Error('missing XMLHttpRequest');
      request.log = {
        'trace': noop,
        'debug': noop,
        'info': noop,
        'warn': noop,
        'error': noop
      };
      var DEFAULT_TIMEOUT = 3 * 60 * 1000;
      // 3 minutes
      //
      // request
      //
      function request(options, callback) {
        // The entry-point to the API: prep the options object and pass the real work to run_xhr.
        if (typeof callback !== 'function')
          throw new Error('Bad callback given: ' + callback);
        if (!options)
          throw new Error('No options given');
        var options_onResponse = options.onResponse;
        // Save this for later.
        if (typeof options === 'string')
          options = { 'uri': options };
        else
          options = JSON.parse(JSON.stringify(options));
        // Use a duplicate for mutating.
        options.onResponse = options_onResponse;
        // And put it back.
        if (options.verbose)
          request.log = getLogger();
        if (options.url) {
          options.uri = options.url;
          delete options.url
        }
        if (!options.uri && options.uri !== '')
          throw new Error('options.uri is a required argument');
        if (typeof options.uri != 'string')
          throw new Error('options.uri must be a string');
        var unsupported_options = [
          'proxy',
          '_redirectsFollowed',
          'maxRedirects',
          'followRedirect'
        ];
        for (var i = 0; i < unsupported_options.length; i++)
          if (options[unsupported_options[i]])
            throw new Error('options.' + unsupported_options[i] + ' is not supported');
        options.callback = callback;
        options.method = options.method || 'GET';
        options.headers = options.headers || {};
        options.body = options.body || null;
        options.timeout = options.timeout || request.DEFAULT_TIMEOUT;
        if (options.headers.host)
          throw new Error('Options.headers.host is not supported');
        if (options.json) {
          options.headers.accept = options.headers.accept || 'application/json';
          if (options.method !== 'GET')
            options.headers['content-type'] = 'application/json';
          if (typeof options.json !== 'boolean')
            options.body = JSON.stringify(options.json);
          else if (typeof options.body !== 'string')
            options.body = JSON.stringify(options.body)
        }
        //BEGIN QS Hack
        var serialize = function (obj) {
          var str = [];
          for (var p in obj)
            if (obj.hasOwnProperty(p)) {
              str.push(encodeURIComponent(p) + '=' + encodeURIComponent(obj[p]))
            }
          return str.join('&')
        };
        if (options.qs) {
          var qs = typeof options.qs == 'string' ? options.qs : serialize(options.qs);
          if (options.uri.indexOf('?') !== -1) {
            //no get params
            options.uri = options.uri + '&' + qs
          } else {
            //existing get params
            options.uri = options.uri + '?' + qs
          }
        }
        //END QS Hack
        //BEGIN FORM Hack
        var multipart = function (obj) {
          //todo: support file type (useful?)
          var result = {};
          result.boundry = '-------------------------------' + Math.floor(Math.random() * 1000000000);
          var lines = [];
          for (var p in obj) {
            if (obj.hasOwnProperty(p)) {
              lines.push('--' + result.boundry + '\n' + 'Content-Disposition: form-data; name="' + p + '"' + '\n' + '\n' + obj[p] + '\n')
            }
          }
          lines.push('--' + result.boundry + '--');
          result.body = lines.join('');
          result.length = result.body.length;
          result.type = 'multipart/form-data; boundary=' + result.boundry;
          return result
        };
        if (options.form) {
          if (typeof options.form == 'string')
            throw 'form name unsupported';
          if (options.method === 'POST') {
            var encoding = (options.encoding || 'application/x-www-form-urlencoded').toLowerCase();
            options.headers['content-type'] = encoding;
            switch (encoding) {
            case 'application/x-www-form-urlencoded':
              options.body = serialize(options.form).replace(/%20/g, '+');
              break;
            case 'multipart/form-data':
              var multi = multipart(options.form);
              //options.headers['content-length'] = multi.length;
              options.body = multi.body;
              options.headers['content-type'] = multi.type;
              break;
            default:
              throw new Error('unsupported encoding:' + encoding)
            }
          }
        }
        //END FORM Hack
        // If onResponse is boolean true, call back immediately when the response is known,
        // not when the full request is complete.
        options.onResponse = options.onResponse || noop;
        if (options.onResponse === true) {
          options.onResponse = callback;
          options.callback = noop
        }
        // XXX Browsers do not like this.
        //if(options.body)
        //  options.headers['content-length'] = options.body.length;
        // HTTP basic authentication
        if (!options.headers.authorization && options.auth)
          options.headers.authorization = 'Basic ' + b64_enc(options.auth.username + ':' + options.auth.password);
        return run_xhr(options)
      }
      var req_seq = 0;
      function run_xhr(options) {
        var xhr = new XHR, timed_out = false, is_cors = is_crossDomain(options.uri), supports_cors = 'withCredentials' in xhr;
        req_seq += 1;
        xhr.seq_id = req_seq;
        xhr.id = req_seq + ': ' + options.method + ' ' + options.uri;
        xhr._id = xhr.id;
        // I know I will type "_id" from habit all the time.
        if (is_cors && !supports_cors) {
          var cors_err = new Error('Browser does not support cross-origin request: ' + options.uri);
          cors_err.cors = 'unsupported';
          return options.callback(cors_err, xhr)
        }
        xhr.timeoutTimer = setTimeout(too_late, options.timeout);
        function too_late() {
          timed_out = true;
          var er = new Error('ETIMEDOUT');
          er.code = 'ETIMEDOUT';
          er.duration = options.timeout;
          request.log.error('Timeout', {
            'id': xhr._id,
            'milliseconds': options.timeout
          });
          return options.callback(er, xhr)
        }
        // Some states can be skipped over, so remember what is still incomplete.
        var did = {
          'response': false,
          'loading': false,
          'end': false
        };
        xhr.onreadystatechange = on_state_change;
        xhr.open(options.method, options.uri, true);
        // asynchronous
        if (is_cors)
          xhr.withCredentials = !!options.withCredentials;
        xhr.send(options.body);
        return xhr;
        function on_state_change(event) {
          if (timed_out)
            return request.log.debug('Ignoring timed out state change', {
              'state': xhr.readyState,
              'id': xhr.id
            });
          request.log.debug('State change', {
            'state': xhr.readyState,
            'id': xhr.id,
            'timed_out': timed_out
          });
          if (xhr.readyState === XHR.OPENED) {
            request.log.debug('Request started', { 'id': xhr.id });
            for (var key in options.headers)
              xhr.setRequestHeader(key, options.headers[key])
          } else if (xhr.readyState === XHR.HEADERS_RECEIVED)
            on_response();
          else if (xhr.readyState === XHR.LOADING) {
            on_response();
            on_loading()
          } else if (xhr.readyState === XHR.DONE) {
            on_response();
            on_loading();
            on_end()
          }
        }
        function on_response() {
          if (did.response)
            return;
          did.response = true;
          request.log.debug('Got response', {
            'id': xhr.id,
            'status': xhr.status
          });
          clearTimeout(xhr.timeoutTimer);
          xhr.statusCode = xhr.status;
          // Node request compatibility
          // Detect failed CORS requests.
          if (is_cors && xhr.statusCode == 0) {
            var cors_err = new Error('CORS request rejected: ' + options.uri);
            cors_err.cors = 'rejected';
            // Do not process this request further.
            did.loading = true;
            did.end = true;
            return options.callback(cors_err, xhr)
          }
          options.onResponse(null, xhr)
        }
        function on_loading() {
          if (did.loading)
            return;
          did.loading = true;
          request.log.debug('Response body loading', { 'id': xhr.id })  // TODO: Maybe simulate "data" events by watching xhr.responseText
        }
        function on_end() {
          if (did.end)
            return;
          did.end = true;
          request.log.debug('Request done', { 'id': xhr.id });
          xhr.body = xhr.responseText;
          if (options.json) {
            try {
              xhr.body = JSON.parse(xhr.responseText)
            } catch (er) {
              return options.callback(er, xhr)
            }
          }
          options.callback(null, xhr, xhr.body)
        }
      }
      // request
      request.withCredentials = false;
      request.DEFAULT_TIMEOUT = DEFAULT_TIMEOUT;
      //
      // defaults
      //
      request.defaults = function (options, requester) {
        var def = function (method) {
          var d = function (params, callback) {
            if (typeof params === 'string')
              params = { 'uri': params };
            else {
              params = JSON.parse(JSON.stringify(params))
            }
            for (var i in options) {
              if (params[i] === undefined)
                params[i] = options[i]
            }
            return method(params, callback)
          };
          return d
        };
        var de = def(request);
        de.get = def(request.get);
        de.post = def(request.post);
        de.put = def(request.put);
        de.head = def(request.head);
        return de
      };
      //
      // HTTP method shortcuts
      //
      var shortcuts = [
        'get',
        'put',
        'post',
        'head'
      ];
      shortcuts.forEach(function (shortcut) {
        var method = shortcut.toUpperCase();
        var func = shortcut.toLowerCase();
        request[func] = function (opts) {
          if (typeof opts === 'string')
            opts = {
              'method': method,
              'uri': opts
            };
          else {
            opts = JSON.parse(JSON.stringify(opts));
            opts.method = method
          }
          var args = [opts].concat(Array.prototype.slice.apply(arguments, [1]));
          return request.apply(this, args)
        }
      });
      //
      // CouchDB shortcut
      //
      request.couch = function (options, callback) {
        if (typeof options === 'string')
          options = { 'uri': options };
        // Just use the request API to do JSON.
        options.json = true;
        if (options.body)
          options.json = options.body;
        delete options.body;
        callback = callback || noop;
        var xhr = request(options, couch_handler);
        return xhr;
        function couch_handler(er, resp, body) {
          if (er)
            return callback(er, resp, body);
          if ((resp.statusCode < 200 || resp.statusCode > 299) && body.error) {
            // The body is a Couch JSON object indicating the error.
            er = new Error('CouchDB error: ' + (body.error.reason || body.error.error));
            for (var key in body)
              er[key] = body[key];
            return callback(er, resp, body)
          }
          return callback(er, resp, body)
        }
      };
      //
      // Utility
      //
      function noop() {
      }
      function getLogger() {
        var logger = {}, levels = [
            'trace',
            'debug',
            'info',
            'warn',
            'error'
          ], level, i;
        for (i = 0; i < levels.length; i++) {
          level = levels[i];
          logger[level] = noop;
          if (typeof console !== 'undefined' && console && console[level])
            logger[level] = formatted(console, level)
        }
        return logger
      }
      function formatted(obj, method) {
        return formatted_logger;
        function formatted_logger(str, context) {
          if (typeof context === 'object')
            str += ' ' + JSON.stringify(context);
          return obj[method].call(obj, str)
        }
      }
      // Return whether a URL is a cross-domain request.
      function is_crossDomain(url) {
        var rurl = /^([\w\+\.\-]+:)(?:\/\/([^\/?#:]*)(?::(\d+))?)?/;
        // jQuery #8138, IE may throw an exception when accessing
        // a field from window.location if document.domain has been set
        var ajaxLocation;
        try {
          ajaxLocation = location.href
        } catch (e) {
          // Use the href attribute of an A element since IE will modify it given document.location
          ajaxLocation = document.createElement('a');
          ajaxLocation.href = '';
          ajaxLocation = ajaxLocation.href
        }
        var ajaxLocParts = rurl.exec(ajaxLocation.toLowerCase()) || [], parts = rurl.exec(url.toLowerCase());
        var result = !!(parts && (parts[1] != ajaxLocParts[1] || parts[2] != ajaxLocParts[2] || (parts[3] || (parts[1] === 'http:' ? 80 : 443)) != (ajaxLocParts[3] || (ajaxLocParts[1] === 'http:' ? 80 : 443))));
        //console.debug('is_crossDomain('+url+') -> ' + result)
        return result
      }
      // MIT License from http://phpjs.org/functions/base64_encode:358
      function b64_enc(data) {
        // Encodes string using MIME base64 algorithm
        var b64 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=';
        var o1, o2, o3, h1, h2, h3, h4, bits, i = 0, ac = 0, enc = '', tmp_arr = [];
        if (!data) {
          return data
        }
        // assume utf8 data
        // data = this.utf8_encode(data+'');
        do {
          // pack three octets into four hexets
          o1 = data.charCodeAt(i++);
          o2 = data.charCodeAt(i++);
          o3 = data.charCodeAt(i++);
          bits = o1 << 16 | o2 << 8 | o3;
          h1 = bits >> 18 & 63;
          h2 = bits >> 12 & 63;
          h3 = bits >> 6 & 63;
          h4 = bits & 63;
          // use hexets to index into b64, and append result to encoded string
          tmp_arr[ac++] = b64.charAt(h1) + b64.charAt(h2) + b64.charAt(h3) + b64.charAt(h4)
        } while (i < data.length);
        enc = tmp_arr.join('');
        switch (data.length % 3) {
        case 1:
          enc = enc.slice(0, -2) + '==';
          break;
        case 2:
          enc = enc.slice(0, -1) + '=';
          break
        }
        return enc
      }
      return request  //UMD FOOTER START
    }))  //UMD FOOTER END
  });
  // source: /Users/dtai/work/verus/crowdstart/assets/js/utils/validation.coffee
  require.define('./Users/dtai/work/verus/crowdstart/assets/js/utils/validation', function (module, exports, __dirname, __filename) {
    exports.isEmpty = function (str) {
      return str.trim().length === 0
    };
    exports.isEmail = function (email) {
      var pattern;
      pattern = new RegExp(/^[+a-zA-Z0-9._-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,4}$/i);
      return pattern.test(email)
    };
    exports.isPassword = function (password, length) {
      return password.length >= length
    };
    exports.passwordsMatch = function (password, confirmPassword) {
      return password === confirmPassword
    };
    exports.error = function (el) {
      $(el).addClass('error');
      $(el).addClass('shake');
      return setTimeout(function () {
        return $(el).removeClass('shake')
      }, 500)
    }
  });
  // source: /Users/dtai/work/verus/crowdstart/assets/js/store/util.coffee
  require.define('./Users/dtai/work/verus/crowdstart/assets/js/store/util', function (module, exports, __dirname, __filename) {
    var humanizeNumber;
    exports.humanizeNumber = humanizeNumber = function (num) {
      return num.toString().replace(/(\d)(?=(\d\d\d)+(?!\d))/g, '$1,')
    };
    exports.formatCurrency = function (num) {
      var currency;
      currency = num || 0;
      return humanizeNumber(currency.toFixed(2))
    };
    exports.numbersOnly = function (event) {
      return event.charCode >= 48 && event.charCode <= 57
    }
  });
  // source: /Users/dtai/work/verus/crowdstart/assets/js/checkout/checkout.coffee
  require.define('./checkout', function (module, exports, __dirname, __filename) {
    var $city, $country, $shipping, $shippingHidden, $state, $subtotal, $tax, $taxHidden, $total, ar1Quantity, card, clearError, updateShippingAndTax, util, validateForm, validation, validator;
    card = require('card/lib/js/card');
    validator = require('address-validator/src/validator');
    validation = require('./Users/dtai/work/verus/crowdstart/assets/js/utils/validation');
    util = require('./Users/dtai/work/verus/crowdstart/assets/js/store/util');
    validateForm = function () {
      var $errors, email, empty, error, errors, j, len, missing, pos, v, valid;
      $errors = $('#error-message');
      $errors.text('');
      valid = true;
      errors = [];
      empty = $('div:visible.required > input').filter(function () {
        return validation.isEmpty($(this).val())
      });
      window.empty = empty;
      email = $('input[name="User.Email"]');
      if (email.length !== 0) {
        if (!validation.isEmail(email.val())) {
          valid = false;
          email.addClass('error');
          email.addClass('shake');
          setTimeout(function () {
            return email.removeClass('shake')
          }, 500);
          errors.push('Invalid email.')
        }
      }
      if (empty.length > 0) {
        valid = false;
        empty.addClass('error');
        empty.addClass('shake');
        setTimeout(function () {
          empty.removeClass('shake')
        }, 500);
        missing = function () {
          var j, len, ref, results;
          ref = empty.parent().text().split('\n');
          results = [];
          for (j = 0, len = ref.length; j < len; j++) {
            v = ref[j];
            if (v.trim()) {
              results.push(v.trim())
            }
          }
          return results
        }();
        errors.push('Missing ' + missing.join(', ') + '.')
      }
      if (!valid) {
        for (j = 0, len = errors.length; j < len; j++) {
          error = errors[j];
          $errors.append($('<p>' + error + '</p>'))
        }
        if (empty.length > 0) {
          location.href = '#' + empty[0].id;
          pos = $(window).scrollTop() - 100;
          location.hash = '';
          $(window).scrollTop(pos)
        }
      }
      return valid
    };
    clearError = function () {
      return $(this).removeClass('error')
    };
    $('div.field input').on('click', clearError);
    $('div.field input').on('change', clearError);
    $('input[name="ShipToBilling"]').change(function () {
      var shipping;
      shipping = $('.shipping-information fieldset');
      if (this.checked) {
        shipping.fadeOut(500);
        setTimeout(function () {
          shipping.css('display', 'none')
        }, 500)
      } else {
        shipping.fadeIn(500);
        shipping.css('display', 'block')
      }
    });
    $state = $('input[name="Order.BillingAddress.State"]');
    $city = $('input[name="Order.BillingAddress.City"]');
    $country = $('input[name="Order.BillingAddress.Country"]');
    $subtotal = $('span.subtotal');
    $tax = $('span.tax');
    $shipping = $('span.shipping');
    $total = $('span.grand-total');
    $taxHidden = $('input[name="Order.Tax"]');
    $shippingHidden = $('input[name="Order.Shipping"]');
    ar1Quantity = 0;
    $('input').filter(function (i, v) {
      return $(v).val() === 'ar-1'
    }).each(function (i, v) {
      var name, selector;
      name = $(v).attr('name').replace('Product.Slug', 'Quantity');
      selector = "input[name='" + name + "']";
      return ar1Quantity += parseInt($(selector).val(), 10)
    });
    updateShippingAndTax = $.debounce(250, function () {
      var city, country, shipping, state, subtotal, tax, total;
      country = $country.val().trim().replace(' ', '');
      city = $city.val().trim();
      state = $state.val().trim();
      subtotal = parseFloat($subtotal.text().replace(',', ''));
      shipping = 0;
      tax = 0;
      total = 0;
      if (!/^usa$|^us$|unitedstates$|unitedstatesofamerica/i.test(country)) {
        shipping = 100 * ar1Quantity
      } else {
        shipping = 0
      }
      if (/^usa$|^us$|unitedstates$|unitedstatesofamerica/i.test(country) && /^ca$|^cali/i.test(state)) {
        tax += subtotal * 0.075;
        if (/san francisco/i.test(city)) {
          tax += subtotal * 0.0125
        }
      } else {
        tax = 0
      }
      total = subtotal + shipping + tax;
      $shipping.text(util.humanizeNumber(shipping.toFixed(2)));
      $tax.text(util.humanizeNumber(tax.toFixed(2)));
      $total.text(util.humanizeNumber(total.toFixed(2)));
      $shippingHidden.val(Math.ceil(shipping * 10000));
      $taxHidden.val(Math.ceil(tax * 10000))
    });
    $state.change(updateShippingAndTax);
    $city.on('keyup', updateShippingAndTax);
    $country.change(updateShippingAndTax);
    $(document).ready(function () {
      var $form, lock, stripeAuthorize, validateBilling, validateShipping;
      $form = $('#form');
      stripeAuthorize = function () {
        app.set('approved', false);
        return function (status, response) {
          var token;
          console.log('Got response from stripe', response);
          if (response.error) {
            $('#error-message').text(response.error.message)
          } else {
            app.set('approved', true);
            token = response.id;
            $('input[name="StripeToken"]').val(token);
            $form.submit()
          }
        }
      }();
      validateBilling = function () {
        var $billingInfo;
        $billingInfo = $('.billing-information');
        $billingInfo.find('input').change(function () {
          return app.set('validBillingAddress', false)
        });
        app.set('validBillingAddress', false);
        return function (err, exact, inexact) {
          var address, alert;
          console.log('Got response from google', arguments);
          if (err == null) {
            if (exact != null && exact.length > 0) {
              address = exact[0]
            } else if (inexact != null && inexact.length > 0) {
              address = inexact[0]
            }
            if (address != null) {
              alert = app.get('alert');
              alert.show({
                cover: true,
                nextTo: $('.billing-information fieldset'),
                title: 'Is this your street address?',
                message: address.toString(),
                confirm: 'Yes',
                onConfirm: function () {
                  $billingInfo.find('#billing-address-1 input').val(address.streetNumber + ' ' + address.street);
                  $billingInfo.find('#billing-city input').val(address.city);
                  $billingInfo.find('#billing-state input').val(address.state);
                  $billingInfo.find('#billing-zip input').val(address.postalCode);
                  $billingInfo.find('#billing-country input').val(address.country);
                  app.set('validBillingAddress', true);
                  return setTimeout(function () {
                    return $form.submit()
                  }, 10)
                },
                cancel: 'No',
                onCancel: function () {
                  return $('#error-message').text('We could not verify your billing address.  Please try again.')
                }
              });
              return true
            }
          }
          $billingInfo.find('input').addClass('error');
          return $('#error-message').text('We could not verify your billing address.  Please try again.')
        }
      }();
      validateShipping = function () {
        var $shippingInfo;
        $shippingInfo = $('.shipping-information');
        $shippingInfo.find('input').change(function () {
          return app.set('validShippingAddress', false)
        });
        app.set('validShippingAddress', false);
        return function (err, exact, inexact) {
          var address, alert;
          console.log('Got response from google', arguments);
          if (err == null) {
            if (exact != null && exact.length > 0) {
              address = exact[0]
            } else if (inexact != null && inexact.length > 0) {
              address = inexact[0]
            }
            if (address != null) {
              alert = app.get('alert');
              alert.show({
                cover: true,
                nextTo: $('.shipping-information fieldset'),
                title: 'Is this your street address?',
                message: address.toString(),
                confirm: 'Yes',
                onConfirm: function () {
                  $shippingInfo.find('#shipping-address-1 input').val(address.streetNumber + ' ' + address.street);
                  $shippingInfo.find('#shipping-city input').val(address.city);
                  $shippingInfo.find('#shipping-state input').val(address.state);
                  $shippingInfo.find('#shipping-zip input').val(address.postalCode);
                  $shippingInfo.find('#shipping-country input').val(address.country);
                  app.set('validShippingAddress', true);
                  return setTimeout(function () {
                    return $form.submit()
                  }, 10)
                },
                cancel: 'No',
                onCancel: function () {
                  return $('#error-message').text('We could not verify your shipping address.  Please try again.')
                }
              });
              return true
            }
          }
          $shippingInfo.find('input').addClass('error');
          return $('#error-message').text('We could not verify your shipping address.  Please try again.')
        }
      }();
      $form.card({
        container: '#card-wrapper',
        numberInput: '#stripe-number',
        expiryInput: '#stripe-expiry-month, #stripe-expiry-year',
        cvcInput: '#stripe-cvc',
        nameInput: '#stripe-name',
        formatting: true,
        values: {
          number: '•••• •••• •••• ••••',
          name: 'Full Name',
          expiry: '••/••',
          cvc: '•••'
        }
      });
      lock = false;
      return $form.submit(function (e) {
        if (!validateForm()) {
          return false
        }
        if (!app.get('approved')) {
          Stripe.card.createToken($form, stripeAuthorize);
          return false
        }
        if (!lock) {
          lock = true;
          $form.find('.btn-container button').append('<div class="loading-spinner" style="float:left"></div>');
          $.ajax({
            url: $form.attr('action'),
            type: 'POST',
            data: $form.serializeArray(),
            dataType: 'json',
            success: function (data) {
              var ref;
              if ((ref = window._fbq) != null) {
                ref.push([
                  'track',
                  '6018312014122',
                  {
                    value: $('.price.grand-total').text(),
                    currency: 'USD'
                  }
                ])
              }
              return window.location.replace('complete/')
            },
            error: function (xhr) {
              var message, ref;
              app.set('approved', false);
              message = xhr != null ? (ref = xhr.responseJSON) != null ? ref.message : void 0 : void 0;
              if (message == null) {
                message = 'We were unable to charge your card. Please review your information and try again later.'
              }
              $('#error-message').text(message);
              $form.find('.loading-spinner').remove();
              return lock = false
            }
          })
        }
        return false
      })
    })
  });
  require('./checkout')
}.call(this, this))