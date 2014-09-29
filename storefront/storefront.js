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
  function require(file, callback) {
    if ({}.hasOwnProperty.call(require.cache, file))
      return require.cache[file];
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
  require.define = function (file, fn) {
    require.modules[file] = fn
  };
  require.define('templates/choice', function (module, exports, __dirname, __filename) {
    jade = require('node_modules/requisite/lib/compilers/jade-runtime');
    module.exports = function anonymous(locals) {
      var buf = [];
      var locals_ = locals || {}, opts = locals_.opts;
      buf.push('<div class="span10"><p>So&comma; we are asking for your understanding while making a choice&colon;</p></div><div class="span5"><div id="choice-ad" class="choice"><div class="choice-number">1</div><div id="choice-ad-body">Keep ' + jade.escape((jade.interp = opts.extension.name) == null ? '' : jade.interp) + ' FREE&period; Let the advertisers pay us&period; It&apos;s not that much&comma; but we&apos;ll make it work&period;</div></div></div><div class="span5"><div id="choice-donate" class="choice"><div class="choice-number">2</div><div id="choice-donate-wrapper"><div id="choice-donate-body">You can turn off ads by&nbsp;<span>making a simple&comma; one-time donation.</span><div id="login-with-paypal"></div></div></div></div></div><div class="span10"><div id="already-donated"><p>Already donated?  Click "Log in with PayPal" to confirm your past donation.</p></div></div>');
      ;
      return buf.join('')
    }
  });
  require.define('node_modules/requisite/lib/compilers/jade-runtime', function (module, exports, __dirname, __filename) {
    if (!Array.isArray) {
      Array.isArray = function (arr) {
        return '[object Array]' == Object.prototype.toString.call(arr)
      }
    }
    if (!Object.keys) {
      Object.keys = function (obj) {
        var arr = [];
        for (var key in obj) {
          if (obj.hasOwnProperty(key)) {
            arr.push(key)
          }
        }
        return arr
      }
    }
    exports.merge = function merge(a, b) {
      var ac = a['class'];
      var bc = b['class'];
      if (ac || bc) {
        ac = ac || [];
        bc = bc || [];
        if (!Array.isArray(ac))
          ac = [ac];
        if (!Array.isArray(bc))
          bc = [bc];
        ac = ac.filter(nulls);
        bc = bc.filter(nulls);
        a['class'] = ac.concat(bc).join(' ')
      }
      for (var key in b) {
        if (key != 'class') {
          a[key] = b[key]
        }
      }
      return a
    };
    function nulls(val) {
      return val != null
    }
    exports.attrs = function attrs(obj, escaped) {
      var buf = [], terse = obj.terse;
      delete obj.terse;
      var keys = Object.keys(obj), len = keys.length;
      if (len) {
        buf.push('');
        for (var i = 0; i < len; ++i) {
          var key = keys[i], val = obj[key];
          if ('boolean' == typeof val || null == val) {
            if (val) {
              terse ? buf.push(key) : buf.push(key + '="' + key + '"')
            }
          } else if (0 == key.indexOf('data') && 'string' != typeof val) {
            buf.push(key + "='" + JSON.stringify(val) + "'")
          } else if ('class' == key && Array.isArray(val)) {
            buf.push(key + '="' + exports.escape(val.join(' ')) + '"')
          } else if (escaped && escaped[key]) {
            buf.push(key + '="' + exports.escape(val) + '"')
          } else {
            buf.push(key + '="' + val + '"')
          }
        }
      }
      return buf.join(' ')
    };
    exports.escape = function escape(html) {
      return String(html).replace(/&(?!(\w+|\#\d+);)/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
    };
    exports.rethrow = function rethrow(err, filename, lineno) {
      throw err
    }
  });
  require.define('templates/upgrade', function (module, exports, __dirname, __filename) {
    jade = require('node_modules/requisite/lib/compilers/jade-runtime');
    module.exports = function anonymous(locals) {
      var buf = [];
      var locals_ = locals || {}, opts = locals_.opts;
      buf.push('<div class="upgrade-headline">' + jade.escape((jade.interp = opts.extension.name) == null ? '' : jade.interp) + ' is now up to date!</div><div class="upgrade-thankyoulink"><a' + jade.attrs({
        'href': opts.thankyou.url,
        'target': '_blank'
      }, {
        'href': true,
        'target': true
      }) + '>' + jade.escape(null == (jade.interp = opts.thankyou.text) ? '' : jade.interp) + "</a></div><p>We started building this project a couple years back and we've been\noverwhelmed by all your feedback!</p><p>Our humble development team (the two of us) is trying to keep updates on pace\nwith the new Chrome versions and building more useful extensions, but it's\ngetting really hard without your help.</p>");
      ;
      return buf.join('')
    }
  });
  require.define('templates/ad-accept', function (module, exports, __dirname, __filename) {
    jade = require('node_modules/requisite/lib/compilers/jade-runtime');
    module.exports = function anonymous(locals) {
      var buf = [];
      buf.push('<p style="text-align:left">You\'ve selected to keep ad support. If you\'d\nlike to turn off ads in the future with a tiny donation, you can do so from\nthe options.</p><p style="text-align:left">Thanks for your loyalty and your help.</p>');
      ;
      return buf.join('')
    }
  });
  require.define('templates/login', function (module, exports, __dirname, __filename) {
    jade = require('node_modules/requisite/lib/compilers/jade-runtime');
    module.exports = function anonymous(locals) {
      var buf = [];
      buf.push('<p style="font-size: 1em">Login with PayPal to complete your donation or verify an existing license.</p><p><a id="retry" href="">Click here</a> to start over.</p>');
      ;
      return buf.join('')
    }
  });
  require.define('templates/checkout', function (module, exports, __dirname, __filename) {
    jade = require('node_modules/requisite/lib/compilers/jade-runtime');
    module.exports = function anonymous(locals) {
      var buf = [];
      var locals_ = locals || {}, opts = locals_.opts;
      buf.push('<div class="row"><div id="instructions">Choose your donation amount using the slider below:</div></div><div class="row slider"><table><tr><td>$' + jade.escape((jade.interp = opts.price.min) == null ? '' : jade.interp) + '</td><td><input' + jade.attrs({
        'id': 'slider',
        'type': 'range',
        'min': '' + opts.price.min + '',
        'max': '' + opts.price.max + '',
        'step': '0.50',
        'value': '' + opts.price.suggested + ''
      }, {
        'type': true,
        'min': true,
        'max': true,
        'step': true,
        'value': true
      }) + '/></td><td>$' + jade.escape((jade.interp = opts.price.max) == null ? '' : jade.interp) + '</td></tr></table></div><div class="row"><div id="amount">Amount:&nbsp;&nbsp;<span id="slider-amount"></span></div></div><div class="row"><div class="checkout-button"><img id="checkout-button" src="https://www.paypal.com/en_US/i/btn/btn_xpressCheckout.gif"/></div></div>');
      ;
      return buf.join('')
    }
  });
  require.define('templates/donated', function (module, exports, __dirname, __filename) {
    jade = require('node_modules/requisite/lib/compilers/jade-runtime');
    module.exports = function anonymous(locals) {
      var buf = [];
      var locals_ = locals || {}, opts = locals_.opts;
      buf.push("<p>Success! Past donation credited. Ad support is now OFF.</p><span>You're our hero!  Please enjoy " + jade.escape((jade.interp = opts.extensionName) == null ? '' : jade.interp) + ' with our gratitude.  Exciting updates on the way!</span>');
      ;
      return buf.join('')
    }
  });
  require.define('templates/loading', function (module, exports, __dirname, __filename) {
    jade = require('node_modules/requisite/lib/compilers/jade-runtime');
    module.exports = function anonymous(locals) {
      var buf = [];
      buf.push('<img style="border: none; background: inherit;" src="/img/loading.gif"/>');
      ;
      return buf.join('')
    }
  });
  require.define('templates/success', function (module, exports, __dirname, __filename) {
    jade = require('node_modules/requisite/lib/compilers/jade-runtime');
    module.exports = function anonymous(locals) {
      var buf = [];
      var locals_ = locals || {}, opts = locals_.opts;
      buf.push("<p>Success! Your donation has been received. Ad support is now OFF.</p><span>You're our hero!  Please enjoy " + jade.escape((jade.interp = opts.extensionName) == null ? '' : jade.interp) + ' with our gratitude.  Exciting updates on the way!</span>');
      ;
      return buf.join('')
    }
  });
  require.define('templates/cancel', function (module, exports, __dirname, __filename) {
    jade = require('node_modules/requisite/lib/compilers/jade-runtime');
    module.exports = function anonymous(locals) {
      var buf = [];
      buf.push('<p>Your donation was not received.</p><span>If this was in error, <a id="retry" href="">click here</a> to try again.</span>');
      ;
      return buf.join('')
    }
  });
  require.define('templates/error', function (module, exports, __dirname, __filename) {
    jade = require('node_modules/requisite/lib/compilers/jade-runtime');
    module.exports = function anonymous(locals) {
      var buf = [];
      buf.push('<p>Oh no! Something went wrong! Our apologies, please <a id="retry" href="">click here</a> to try again later.</p>');
      ;
      return buf.join('')
    }
  });
  require.define('storefront.coffee', function (module, exports, __dirname, __filename) {
    var AdView, Application, CancelView, CheckoutView, ChoiceView, DonatedView, ErrorView, LoadingView, LoginView, Router, StaticView, SuccessView, UpgradeView, User, updateDonateButton, _ref, _ref1, _ref10, _ref11, _ref12, _ref2, _ref3, _ref4, _ref5, _ref6, _ref7, _ref8, _ref9, __hasProp = {}.hasOwnProperty, __extends = function (child, parent) {
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
    updateDonateButton = function (title, message) {
      $('#donate-button-lg .donate-here').text(title);
      return $('#donate-button-lg .turn-off-ads').text(message)
    };
    Application = function () {
      function Application(opts) {
        this.opts = opts
      }
      Application.prototype.start = function () {
        this.router = new Router;
        return Backbone.history.start()
      };
      Application.prototype.navigate = function (url) {
        this.router.navigate('navigating...', { trigger: true });
        return this.router.navigate(url, { trigger: true })
      };
      Application.prototype.loggedIn = function (id, code) {
        var _this = this;
        console.log('logged in', id, code);
        window.user = this.user = new User({
          id: id,
          code: code
        });
        this.swapView(new LoadingView);
        return this.user.fetch({
          success: function () {
            if (_this.user.get('purchased')) {
              return _this.swapView(new DonatedView)
            } else {
              return _this.swapView(new CheckoutView)
            }
          },
          error: function () {
            return console.log('ERROR: Failed to fetch user data')
          }
        })
      };
      Application.prototype.swapView = function (view) {
        return $('#donate-action').html('').append(view.el)
      };
      Application.prototype.startCheckout = function (amount, currency) {
        var url, _this = this;
        url = 'http://' + this.opts.kachingUrl + '/api/v1/start-checkout?amount=' + amount + '&currency=' + currency + '&email=' + this.user.get('email') + '&id=' + this.opts.extension.id;
        return $.getJSON(url, function (data) {
          var token;
          console.log(data);
          token = data.token;
          return window.location.href = 'https://' + _this.opts.paypal.expressCheckoutEndpoint + '/cgi-bin/webscr?cmd=_express-checkout&token=' + token
        })
      };
      Application.prototype.fireDonated = function (data) {
        var id;
        return id = setInterval(function () {
          var el, ev;
          el = document.getElementById('donated-event');
          if (el != null) {
            clearInterval(id);
            console.log('firing donated event', data);
            ev = document.createEvent('Event');
            ev.initEvent('donated', true, true);
            el.innerText = data;
            return el.dispatchEvent(ev)
          }
        }, 100)
      };
      return Application
    }();
    User = function (_super) {
      __extends(User, _super);
      function User() {
        _ref = User.__super__.constructor.apply(this, arguments);
        return _ref
      }
      User.prototype.url = function () {
        return '' + this.urlRoot() + '?code=' + this.get('code') + '&id=' + this.get('id')
      };
      User.prototype.urlRoot = function () {
        return 'http://' + app.opts.kachingUrl + '/api/v1/get-user-info'
      };
      return User
    }(Backbone.Model);
    StaticView = function (_super) {
      __extends(StaticView, _super);
      function StaticView() {
        _ref1 = StaticView.__super__.constructor.apply(this, arguments);
        return _ref1
      }
      StaticView.prototype.initialize = function () {
        return this.render()
      };
      StaticView.prototype.render = function () {
        this.$el.html(this.template({ opts: app.opts }));
        return this
      };
      return StaticView
    }(Backbone.View);
    ChoiceView = function (_super) {
      __extends(ChoiceView, _super);
      function ChoiceView() {
        _ref2 = ChoiceView.__super__.constructor.apply(this, arguments);
        return _ref2
      }
      ChoiceView.prototype.id = 'choice-view';
      ChoiceView.prototype.template = require('templates/choice');
      ChoiceView.prototype.initialize = function () {
        ChoiceView.__super__.initialize.apply(this, arguments);
        return paypal.use(['login'], function (login) {
          return login.render(app.opts.paypal.loginButton)
        })
      };
      ChoiceView.prototype.events = {
        'click #choice-ad': 'choiceAd',
        'click #login-with-paypal': 'choiceDonate'
      };
      ChoiceView.prototype.choiceAd = function (e) {
        $('#donate-plea').html('');
        return app.swapView(new AdView)
      };
      ChoiceView.prototype.choiceDonate = function (e) {
        return app.swapView(new LoginView)
      };
      return ChoiceView
    }(StaticView);
    UpgradeView = function (_super) {
      __extends(UpgradeView, _super);
      function UpgradeView() {
        _ref3 = UpgradeView.__super__.constructor.apply(this, arguments);
        return _ref3
      }
      UpgradeView.prototype.className = 'upgrade-view';
      UpgradeView.prototype.template = require('templates/upgrade');
      UpgradeView.prototype.render = function () {
        UpgradeView.__super__.render.apply(this, arguments);
        return $('#donate-headline').html('')
      };
      return UpgradeView
    }(StaticView);
    AdView = function (_super) {
      __extends(AdView, _super);
      function AdView() {
        _ref4 = AdView.__super__.constructor.apply(this, arguments);
        return _ref4
      }
      AdView.prototype.className = 'span10';
      AdView.prototype.template = require('templates/ad-accept');
      AdView.prototype.render = function () {
        AdView.__super__.render.apply(this, arguments);
        return updateDonateButton('THANK YOU', 'Ad support is now on')
      };
      return AdView
    }(StaticView);
    LoginView = function (_super) {
      __extends(LoginView, _super);
      function LoginView() {
        _ref5 = LoginView.__super__.constructor.apply(this, arguments);
        return _ref5
      }
      LoginView.prototype.className = 'span10';
      LoginView.prototype.template = require('templates/login');
      LoginView.prototype.events = {
        'click #retry': function (ev) {
          app.navigate('/');
          return ev.preventDefault()
        }
      };
      return LoginView
    }(StaticView);
    CheckoutView = function (_super) {
      __extends(CheckoutView, _super);
      function CheckoutView() {
        _ref6 = CheckoutView.__super__.constructor.apply(this, arguments);
        return _ref6
      }
      CheckoutView.prototype.className = 'span10';
      CheckoutView.prototype.template = require('templates/checkout');
      CheckoutView.prototype.events = {
        'change input#slider': 'onSliderInput',
        'click #checkout-button': 'onCheckoutButtonClick'
      };
      CheckoutView.prototype.render = function () {
        CheckoutView.__super__.render.apply(this, arguments);
        this.slider = this.$el.find('input#slider');
        this.sliderAmount = this.$el.find('#slider-amount');
        return this.onSliderInput()
      };
      CheckoutView.prototype.onSliderInput = function () {
        return this.sliderAmount.text(this.formatCurrency(this.slider.val()))
      };
      CheckoutView.prototype.onCheckoutButtonClick = function () {
        var amount, currency;
        amount = this.slider.val();
        currency = 'USD';
        return app.startCheckout(amount, currency)
      };
      CheckoutView.prototype.formatCurrency = function (value) {
        return '$' + parseFloat(value).toFixed(2).toString()
      };
      return CheckoutView
    }(StaticView);
    DonatedView = function (_super) {
      __extends(DonatedView, _super);
      function DonatedView() {
        _ref7 = DonatedView.__super__.constructor.apply(this, arguments);
        return _ref7
      }
      DonatedView.prototype.className = 'span10 donated-view';
      DonatedView.prototype.template = require('templates/donated');
      DonatedView.prototype.render = function () {
        DonatedView.__super__.render.apply(this, arguments);
        updateDonateButton('THANK YOU', 'Ad support is now off');
        $('#donate-plea').html('');
        return app.fireDonated('donated')
      };
      return DonatedView
    }(StaticView);
    LoadingView = function (_super) {
      __extends(LoadingView, _super);
      function LoadingView() {
        _ref8 = LoadingView.__super__.constructor.apply(this, arguments);
        return _ref8
      }
      LoadingView.prototype.className = 'span10';
      LoadingView.prototype.template = require('templates/loading');
      return LoadingView
    }(StaticView);
    SuccessView = function (_super) {
      __extends(SuccessView, _super);
      function SuccessView() {
        _ref9 = SuccessView.__super__.constructor.apply(this, arguments);
        return _ref9
      }
      SuccessView.prototype.className = 'span10 success-view';
      SuccessView.prototype.template = require('templates/success');
      SuccessView.prototype.render = function () {
        SuccessView.__super__.render.apply(this, arguments);
        updateDonateButton('THANK YOU', 'Ad support is now off');
        $('#donate-plea').html('');
        return app.fireDonated('success')
      };
      return SuccessView
    }(StaticView);
    CancelView = function (_super) {
      __extends(CancelView, _super);
      function CancelView() {
        _ref10 = CancelView.__super__.constructor.apply(this, arguments);
        return _ref10
      }
      CancelView.prototype.className = 'span10 cancel-view';
      CancelView.prototype.template = require('templates/cancel');
      CancelView.prototype.events = { 'click #retry': 'onRetry' };
      CancelView.prototype.onRetry = function (ev) {
        app.navigate('/');
        return ev.preventDefault()
      };
      return CancelView
    }(StaticView);
    ErrorView = function (_super) {
      __extends(ErrorView, _super);
      function ErrorView() {
        _ref11 = ErrorView.__super__.constructor.apply(this, arguments);
        return _ref11
      }
      ErrorView.prototype.className = 'span10';
      ErrorView.prototype.template = require('templates/error');
      ErrorView.prototype.events = { 'click #retry': 'onRetry' };
      ErrorView.prototype.onRetry = function (ev) {
        app.navigate('/');
        return ev.preventDefault()
      };
      return ErrorView
    }(StaticView);
    Router = function (_super) {
      __extends(Router, _super);
      function Router() {
        _ref12 = Router.__super__.constructor.apply(this, arguments);
        return _ref12
      }
      Router.prototype.routes = {
        '': 'choice',
        'cancel?*q': 'cancel',
        'checkout': 'checkout',
        'error': 'error',
        'success': 'success',
        'upgrade': 'upgrade'
      };
      Router.prototype.choice = function () {
        return app.swapView(new ChoiceView)
      };
      Router.prototype.checkout = function () {
        return app.swapView(new CheckoutView)
      };
      Router.prototype.success = function () {
        return app.swapView(new SuccessView)
      };
      Router.prototype.cancel = function () {
        return app.swapView(new CancelView)
      };
      Router.prototype.error = function () {
        return app.swapView(new ErrorView)
      };
      Router.prototype.upgrade = function () {
        var view;
        app.swapView(new ChoiceView);
        return view = new UpgradeView({ el: $('#one') })
      };
      return Router
    }(Backbone.Router);
    window.storefrontStart = function (opts) {
      window.app = new Application(opts);
      return app.start()
    };
    window.storefront = {
      start: function (opts) {
        window.app = new Application(opts);
        return app.start()
      },
      UpgradeView: UpgradeView
    }
  });
  require('storefront.coffee')
}.call(this, this))
