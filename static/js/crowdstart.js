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
    var $slides, formatCurrency, humanizeNumber;
    humanizeNumber = function (num) {
      return num.toString().replace(/(\d)(?=(\d\d\d)+(?!\d))/g, '$1,')
    };
    formatCurrency = function (num) {
      var currency;
      currency = num || 0;
      return humanizeNumber(currency.toFixed(2))
    };
    window.csio = window.csio || {};
    csio.cookieName = 'SKULLYSystemsCart';
    $.cookie.json = true;
    csio.Alert = function (opts) {
      var $el, dismiss, offset, topOffset;
      dismiss = function () {
        $el.fadeOut(200, function () {
          $el.css({ top: -1000 })
        })
      };
      $el = $('.sqs-widgets-confirmation.alert');
      offset = opts.$nextTo.offset();
      topOffset = offset.top - $(window).scrollTop();
      $el.find('.title').text(opts.title);
      $el.find('.message').text(opts.message);
      $el.find('.confirmation-button').text(opts.confirm);
      $el.css({
        position: 'fixed',
        top: topOffset - 42 + 'px',
        left: offset.left - 66 + 'px'
      });
      $el.fadeIn(200);
      $(document).mousedown(function () {
        dismiss()
      });
      $(document).keydown(function (e) {
        if (!e) {
          e = event
        }
        if (e.keyCode === 27) {
          dismiss()
        }
      });
      $(window).scroll(function () {
        dismiss()
      })
    };
    csio.getVariant = function () {
      var i, missingOptions, optionsMatch, selected, variant, variants;
      optionsMatch = function (selected, variant) {
        var k;
        for (k in selected) {
          continue
        }
        return true
      };
      selected = {};
      variants = csio.currentProduct.Variants;
      missingOptions = [];
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
        return csio.Alert({
          title: 'Unable To Add Item',
          message: 'Please select a ' + missingOptions[0] + ' option.',
          confirm: 'Okay',
          $nextTo: $('.sqs-add-to-cart-button')
        })
      }
      i = 0;
      while (i < variants.length) {
        variant = variants[i];
        if (optionsMatch(selected, variant)) {
          return variant
        }
        i++
      }
      return variants[0]
    };
    csio.addToCart = function () {
      var cart, inner, quantity, sku, variant;
      quantity = parseInt($('#quantity').val(), 10);
      cart = $.cookie(csio.cookieName) || {};
      variant = csio.getVariant();
      if (variant == null) {
        return
      }
      sku = variant.sku;
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
        $('.status-text').text('Added!').fadeOut(500, function () {
          inner.html('Add to Cart')
        })
      }, 500);
      setTimeout(function () {
        $('.sqs-pill-shopping-cart-content').animate({ opacity: 0.85 }, 400, function () {
          csio.updateCartHover(cart);
          $('.sqs-pill-shopping-cart-content').animate({ opacity: 1 }, 300)
        })
      }, 300)
    };
    csio.setCart = function (cart) {
      $.cookie(csio.cookieName, cart, {
        expires: 30,
        path: '/'
      })
    };
    csio.getCart = function () {
      return $.cookie(csio.cookieName) || {}
    };
    csio.clearCart = function () {
      $.cookie(csio.cookieName, {}, {
        expires: 30,
        path: '/'
      })
    };
    csio.updateCartHover = function (modifiedCart) {
      var cart, k, lineItem, numItems, subTotal;
      cart = modifiedCart || csio.getCart();
      numItems = 0;
      subTotal = 0;
      for (k in cart) {
        lineItem = cart[k];
        numItems += lineItem.quantity;
        subTotal += lineItem.price * lineItem.quantity
      }
      $('.total-quantity').text(humanizeNumber(numItems));
      $('.subtotal .price span').text(formatCurrency(subTotal));
      if (numItems === 1) {
        $('.details span.suffix').text('item')
      } else {
        $('.details span.suffix').text('items')
      }
    };
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
    }
  });
  require('crowdstart')
}.call(this, this))
