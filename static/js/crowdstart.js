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
  require.define('crowdstart', function (module, exports, __dirname, __filename) {
    var $slides, alert, util;
    alert = require('./alert');
    util = require('./util');
    window.csio = window.csio || {};
    csio.cookieName = 'SKULLYSystemsCart';
    $.cookie.json = true;
    $('.fixed-cart').click(function () {
      window.location = '/cart'
    });
    $('#productThumbnails .slide img').each(function (i, v) {
      $(v).click(function () {
        var src;
        src = $(v).data('src');
        $('#productSlideshow .slide img').each(function (i, v) {
          if (src === $(v).data('src')) {
            $(v).fadeIn(400)
          } else {
            $(v).fadeOut(400)
          }
        })
      })
    });
    csio.updateCartHover();
    if (location.pathname === '/cart') {
      $('.fixed-cart').hide()
    }
    if (location.pathname === '/products/ar-1') {
      $slides = $('#productSlideshow .slide img');
      $('[data-variant-option-name=Color]').change(function () {
        if ($(this).val() === 'Black') {
          $($slides[0]).fadeIn();
          $($slides[1]).fadeOut()
        } else {
          $($slides[1]).fadeIn();
          $($slides[0]).fadeOut()
        }
      })
    }
    csio.NumbersOnly = function (event) {
      return event.charCode >= 48 && event.charCode <= 57
    };
    require('./cart');
    require('./checkout')
  });
  require('crowdstart')
}.call(this, this))