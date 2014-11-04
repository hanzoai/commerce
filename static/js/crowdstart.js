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
    }
  });
  require.define('./cart', function (module, exports, __dirname, __filename) {
    var product, templateEl;
    product = require('./product');
    if (window.csio == null) {
      window.csio = {}
    }
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
    exports.updateCartHover = function (modifiedCart) {
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
    window.csio = window.csio || {};
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
    var Alert;
    Alert = require('./alert');
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
        return Alert({
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
  require.define('./checkout', function (module, exports, __dirname, __filename) {
    var $city, $requiredVisible, $state, $subtotal, $tax, $total, CheckoutForm, setupCard, showPaymentOptions, updateTax, validation;
    window.csio = window.csio || {};
    validation = {
      isEmpty: function (str) {
        return str.trim().length === 0
      },
      isEmail: function (email) {
        var pattern;
        pattern = new RegExp(/^[+a-zA-Z0-9._-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,4}$/i);
        return pattern.test(email)
      }
    };
    $('div.field').on('click', function () {
      $(this).removeClass('error')
    });
    CheckoutForm = function () {
      function CheckoutForm(selector) {
        this.bindForm(selector)
      }
      CheckoutForm.prototype.bindForm = function () {
        return $('#form').submit(this.submit)
      };
      CheckoutForm.prototype.empty = function () {
        return $('div:visible.required > input').filter(function () {
          return $(this).val() === ''
        })
      };
      CheckoutForm.prototype.submit = function (e) {
        var email, empty;
        empty = this.empty();
        email = $('input[name="User.Email"]');
        if (!validation.isEmail(email.val())) {
          console.log(validation.isEmail(email.text()));
          e.preventDefault();
          email.parent().addClass('error');
          email.parent().addClass('shake');
          setTimeout(function () {
            email.parent().removeClass('shake')
          }, 500)
        }
        if (empty.length > 0) {
          e.preventDefault();
          empty.parent().addClass('error');
          empty.parent().addClass('shake');
          return setTimeout(function () {
            empty.parent().removeClass('shake')
          }, 500)
        }
      };
      return CheckoutForm
    }();
    $requiredVisible = $('div:visible.required > input');
    showPaymentOptions = $.debounce(250, function () {
      var fieldset, i;
      i = 0;
      while (i < $requiredVisible.length) {
        if ($requiredVisible[i].value === '') {
          return
        }
        i++
      }
      fieldset = $('div.sqs-checkout-form-payment-content > fieldset');
      fieldset.css('display', 'block');
      fieldset.css('opacity', '0');
      fieldset.fadeTo(1000, 1);
      $requiredVisible.off('keyup', showPaymentOptions)
    });
    $requiredVisible.on('keyup', showPaymentOptions);
    setupCard = function (selector) {
      return $(selector).card({
        container: '#card-wrapper',
        numberInput: 'input[name="Order.Account.Number"]',
        expiryInput: 'input[name="RawExpiry"]',
        cvcInput: 'input[name="Order.Account.CVV2"]',
        nameInput: 'input[name="User.FirstName"], input[name="User.LastName"]'
      })
    };
    $('input[name="ShipToBilling"]').change(function () {
      var shipping;
      shipping = $('#shippingInfo');
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
    $state = $('select[name="Order.BillingAddress.State"]');
    $city = $('input[name="Order.BillingAddress.City"]');
    $tax = $('div.tax.total > div.price > span');
    $total = $('div.grand-total.total > div.price > span');
    $subtotal = $('div.subtotal.total > div.price > span');
    updateTax = $.debounce(250, function () {
      var city, state, subtotal, tax, total;
      city = $city.val();
      state = $state.val();
      tax = 0;
      total = 0;
      subtotal = parseFloat($subtotal.text().replace(',', ''));
      if (state === 'CA') {
        tax += subtotal * 0.075
      }
      if (state === 'CA' && /san francisco/i.test(city)) {
        tax += subtotal * 0.0125
      }
      total = subtotal + tax;
      $tax.text(tax.toFixed(2));
      $total.text(total.toFixed(2))
    });
    $state.change(updateTax);
    $city.on('keyup', updateTax);
    csio.handleSubmit = function (formSelector) {
      var $message, authorizePending, url;
      $message = $('#authorize-message');
      url = '/checkout/authorize';
      authorizePending = false;
      $(formSelector).submit(function (e) {
        e.preventDefault();
        if (authorizePending) {
          return
        }
        authorizePending = true;
        $.ajax({
          type: 'POST',
          url: url,
          data: $(formSelector).serialize(),
          dataType: 'json',
          error: function (xhr) {
            var data;
            data = $.parseJSON(xhr);
            console.log(data);
            $message.text('Unable to authorize your payment. Please try again in a few moments.');
            $message.fadeIn()
          },
          success: function (data) {
            console.log(data);
            switch (data.status) {
            case 'ok':
              $message.text('Thank you for your payment.');
              break;
            case 'retry':
              $message.text('We were unable to authorize payment, please try again.');
              break;
            case 'declined':
              $message.text('Unable to authorize payment, please check your card details and try again.')
            }
            $message.fadeIn()
          },
          complete: function () {
            authorizePending = false
          },
          timeout: 5000
        })
      })
    };
    csio.handleSubmit('#form')
  });
  require.define('./crowdstart', function (module, exports, __dirname, __filename) {
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
  require('./crowdstart')
}.call(this, this))