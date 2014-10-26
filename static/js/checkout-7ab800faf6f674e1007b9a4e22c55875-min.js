YUI.add("slider-base", function(b, c) {
  function a() {
    a.superclass.constructor.apply(this, arguments)
  }
  var d = b.Attribute.INVALID_VALUE;
  b.SliderBase = b.extend(a, b.Widget, {
    initializer: function() {
      this.axis = this.get("axis");
      this._key = {
        dim: "y" === this.axis ? "height" : "width",
        minEdge: "y" === this.axis ? "top" : "left",
        maxEdge: "y" === this.axis ? "bottom" : "right",
        xyIndex: "y" === this.axis ? 1 : 0
      };
      this.publish("thumbMove", {
        defaultFn: this._defThumbMoveFn,
        queuable: !0
      })
    },
    renderUI: function() {
      var a = this.get("contentBox");
      this.rail = this.renderRail();
      this._uiSetRailLength(this.get("length"));
      this.thumb = this.renderThumb();
      this.rail.appendChild(this.thumb);
      a.appendChild(this.rail);
      a.addClass(this.getClassName(this.axis))
    },
    renderRail: function() {
      var a = this.getClassName("rail", "cap", this._key.minEdge),
        d = this.getClassName("rail", "cap", this._key.maxEdge);
      return b.Node.create(b.Lang.sub(this.RAIL_TEMPLATE, {
        railClass: this.getClassName("rail"),
        railMinCapClass: a,
        railMaxCapClass: d
      }))
    },
    _uiSetRailLength: function(a) {
      this.rail.setStyle(this._key.dim, a)
    },
    renderThumb: function() {
      this._initThumbUrl();
      var a = this.get("thumbUrl");
      return b.Node.create(b.Lang.sub(this.THUMB_TEMPLATE, {
        thumbClass: this.getClassName("thumb"),
        thumbShadowClass: this.getClassName("thumb", "shadow"),
        thumbImageClass: this.getClassName("thumb", "image"),
        thumbShadowUrl: a,
        thumbImageUrl: a,
        thumbAriaLabelId: this.getClassName("label", b.guid())
      }))
    },
    _onThumbClick: function(a) {
      this.thumb.focus()
    },
    bindUI: function() {
      var a = this.get("boundingBox"),
        d = !b.UA.opera ? "down:" : "press:",
        c = d + "37,39",
        h = d + "37+meta,39+meta";
      a.on("key", this._onDirectionKey,
      d + "38,40,33,34,35,36", this);
      a.on("key", this._onLeftRightKey, c, this);
      a.on("key", this._onLeftRightKeyMeta, h, this);
      this.thumb.on("click", this._onThumbClick, this);
      this._bindThumbDD();
      this._bindValueLogic();
      this.after("disabledChange", this._afterDisabledChange);
      this.after("lengthChange", this._afterLengthChange)
    },
    _incrMinor: function() {
      this.set("value", this.get("value") + this.get("minorStep"))
    },
    _decrMinor: function() {
      this.set("value", this.get("value") - this.get("minorStep"))
    },
    _incrMajor: function() {
      this.set("value",
      this.get("value") + this.get("majorStep"))
    },
    _decrMajor: function() {
      this.set("value", this.get("value") - this.get("majorStep"))
    },
    _setToMin: function(a) {
      this.set("value", this.get("min"))
    },
    _setToMax: function(a) {
      this.set("value", this.get("max"))
    },
    _onDirectionKey: function(a) {
      a.preventDefault();
      if (!1 === this.get("disabled")) switch (a.charCode) {
      case 38:
        this._incrMinor();
        break;
      case 40:
        this._decrMinor();
        break;
      case 36:
        this._setToMin();
        break;
      case 35:
        this._setToMax();
        break;
      case 33:
        this._incrMajor();
        break;
      case 34:
        this._decrMajor()
      }
    },
    _onLeftRightKey: function(a) {
      a.preventDefault();
      if (!1 === this.get("disabled")) switch (a.charCode) {
      case 37:
        this._decrMinor();
        break;
      case 39:
        this._incrMinor()
      }
    },
    _onLeftRightKeyMeta: function(a) {
      a.preventDefault();
      if (!1 === this.get("disabled")) switch (a.charCode) {
      case 37:
        this._setToMin();
        break;
      case 39:
        this._setToMax()
      }
    },
    _bindThumbDD: function() {
      var a = {
        constrain: this.rail
      };
      a["stick" + this.axis.toUpperCase()] = !0;
      this._dd = new b.DD.Drag({
        node: this.thumb,
        bubble: !1,
        on: {
          "drag:start": b.bind(this._onDragStart, this)
        },
        after: {
          "drag:drag": b.bind(this._afterDrag, this),
          "drag:end": b.bind(this._afterDragEnd, this)
        }
      });
      this._dd.plug(b.Plugin.DDConstrained, a)
    },
    _bindValueLogic: function() {},
    _uiMoveThumb: function(a, b) {
      this.thumb && (this.thumb.setStyle(this._key.minEdge, a + "px"), b || (b = {}), b.offset = a, this.fire("thumbMove", b))
    },
    _onDragStart: function(a) {
      this.fire("slideStart", {
        ddEvent: a,
        originEvent: a
      })
    },
    _afterDrag: function(a) {
      this.fire("thumbMove", {
        offset: a.info.xy[this._key.xyIndex] - a.target.con._regionCache[this._key.minEdge],
        ddEvent: a,
        originEvent: a
      })
    },
    _afterDragEnd: function(a) {
      this.fire("slideEnd", {
        ddEvent: a,
        originEvent: a
      })
    },
    _afterDisabledChange: function(a) {
      this._dd.set("lock", a.newVal)
    },
    _afterLengthChange: function(a) {
      this.get("rendered") && (this._uiSetRailLength(a.newVal), this.syncUI())
    },
    syncUI: function() {
      this._dd.con.resetCache();
      this._syncThumbPosition();
      this.thumb.set("aria-valuemin", this.get("min"));
      this.thumb.set("aria-valuemax", this.get("max"));
      this._dd.set("lock", this.get("disabled"))
    },
    _syncThumbPosition: function() {},
    _setAxis: function(a) {
      a = (a + "").toLowerCase();
      return "x" === a || "y" === a ? a : d
    },
    _setLength: function(a) {
      a = (a + "").toLowerCase();
      var b = parseFloat(a, 10);
      a = a.replace(/[\d\.\-]/g, "") || this.DEF_UNIT;
      return 0 < b ? b + a : d
    },
    _initThumbUrl: function() {
      if (!this.get("thumbUrl")) {
        var a = this.getSkinName() || "sam",
          d = b.config.base;
        0 === d.indexOf("http://yui.yahooapis.com/combo") && (d = "http://yui.yahooapis.com/" + b.version + "/build/");
        this.set("thumbUrl", d + "slider-base/assets/skins/" + a + "/thumb-" + this.axis + ".png")
      }
    },
    BOUNDING_TEMPLATE: "<span></span>",
    CONTENT_TEMPLATE: "<span></span>",
    RAIL_TEMPLATE: '<span class="{railClass}"><span class="{railMinCapClass}"></span><span class="{railMaxCapClass}"></span></span>',
    THUMB_TEMPLATE: '<span class="{thumbClass}" aria-labelledby="{thumbAriaLabelId}" aria-valuetext="" aria-valuemax="" aria-valuemin="" aria-valuenow="" role="slider" tabindex="0"><img src="{thumbShadowUrl}" alt="Slider thumb shadow" class="{thumbShadowClass}"><img src="{thumbImageUrl}" alt="Slider thumb" class="{thumbImageClass}"></span>'
  }, {
    NAME: "sliderBase",
    ATTRS: {
      axis: {
        value: "x",
        writeOnce: !0,
        setter: "_setAxis",
        lazyAdd: !1
      },
      length: {
        value: "150px",
        setter: "_setLength"
      },
      thumbUrl: {
        value: null,
        validator: b.Lang.isString
      }
    }
  })
}, "3.17.2", {
  requires: ["widget", "dd-constrain", "event-key"],
  skinnable: !0
});
YUI.add("squarespace-dialog-field-2", function(b) {
  b.namespace("Squarespace.Widgets").DialogField2 = b.namespace("Squarespace").DialogField2 = b.Base.create("dialogField", b.Squarespace.Widgets.DataWidget, [b.Squarespace.DialogFieldLegacyInterface, b.WidgetParent, b.WidgetChild], {
    initializer: function(b) {
      this._saveInitialData(b ? b.data : null)
    },
    destructor: function() {
      this._destroyError();
      this.unplug(b.Plugin.WidgetAnim);
      this._initialData = void 0;
      this._errorFlyoutSub && this._errorFlyoutSub.detach()
    },
    _destroyError: function() {
      this._errorNode && (this._errorNode.remove(!0), this._errorNode = null)
    },
    _saveInitialData: function(c) {
      c = !b.Lang.isValue(c) ? this.get("data") : c;
      b.Lang.isArray(c) && this.get("cloneInitialData") ? this._initialData = c.slice() : b.Lang.isObject(c) && this.get("cloneInitialData") ? this._initialData = b.clone(c, !0) : this._initialData = c
    },
    setCurrentDataAsInitial: function() {
      this._saveInitialData()
    },
    renderUI: function() {
      b.Squarespace.DialogField2.superclass.renderUI.call(this);
      var c = this.get("name");
      c && this.get("boundingBox").addClass("name-" + c)
    },
    scrollIntoView: function() {
      this.get("contentBox").scrollIntoView()
    },
    showError: function(c) {
      if (c && this.get("rendered")) {
        var a = this.get("boundingBox"),
          d = this.get("contentBox"),
          e = this.get("errorFlyoutAnchor") || d,
          d = this.get("errorFlyoutAnimationTime");
        e.hasPlugin("flyoutPlugin") || e.plug(b.Squarespace.Animations.Flyout, {
          duration: d,
          renderTarget: this.get("errorFlyoutRenderTarget")
        });
        var f = e.flyoutPlugin,
          g = b.bind(function() {
            a.addClass("error");
            this._destroyError();
            this._errorNode = b.Node.create('<div class="sqs-flyout-error-message">' + c + "</div>");
            var d = this.get("dialog");
            d && this._errorNode.setStyle("zIndex", d.zIndex + 10);
            this._errorNode.on(this.get("id") + "|click", function(a) {
              a.halt();
              this.hideError()
            }, this);
            var g = b.one(window).get("region"),
              l = b.Squarespace.Utils.measureNode(this._errorNode).width,
              d = e.get("region");
            d.right + l <= g.right ? (g = "rt", this._errorNode.addClass("out-from-right")) : d.left - l > g.left ? (g = "lt", this._errorNode.addClass("out-from-left")) : (g = "bl", this._errorNode.addClass("out-from-bottom"), this._errorNode.setStyle("width",
            d.width));
            f.setAttrs({
              node: this._errorNode,
              alignment: g
            });
            this._clearErrorSub = b.on(this.get("id") + "|click", this.hideError, this);
            d = this.get("id");
            f.once(d + "|shown", function(a) {
              this.fire("error-shown")
            }, this);
            this._errorFlyoutSub = e.once("flyout-shown", function(a) {
              this._errorNode.setStyle("width", "100%")
            }, this);
            f.show()
          }, this);
        f.get("visible") ? (this._isHiding = !0, this._showErrorSub && this._showErrorSub.detach(), this._showErrorSub = f.once(this.get("id") + "|hidden", function() {
          this._showErrorSub = null;
          this._isHiding = !1;
          g()
        }, this), f.hide()) : g()
      }
    },
    hideError: function() {
      if (this.get("rendered") && !this._isHiding) {
        var b = this.get("boundingBox"),
          a = this.get("contentBox"),
          a = this.get("errorFlyoutAnchor") || a;
        a.hasPlugin("flyoutPlugin") && a.flyoutPlugin.get("visible") && (this._clearErrorSub && (this._clearErrorSub.detach(), this._clearErrorSub = null), a = a.flyoutPlugin, a.once(this.get("id") + "|hidden", function(a) {
          b.removeClass("error");
          this.fire("error-hidden");
          this._destroyError()
        }, this), a.hide())
      }
    },
    didDataChange: function() {
      var c = this.get("data");
      if (this.get("readOnly")) return !1;
      if (b.Lang.isArray(this._initialData)) return b.JSON.stringify(this._initialData) !== b.JSON.stringify(c);
      if (b.Lang.isObject(this._initialData)) throw "DialogField base class will not compare objects. Define didDataChange for this field.";
      return this._initialData !== c
    },
    isEmpty: function() {
      var b = this.get("data");
      return "" === b || 0 === b
    },
    revert: function() {
      this.set("data", this._initialData, {
        revert: !0
      })
    },
    _getErrors: function() {
      return []
    },
    isValid: function() {
      return !b.Lang.isArray(this.get("errors")) || 0 === this.get("errors").length
    }
  }, {
    CSS_PREFIX: "sqs-dialog-field",
    ATTRS: {
      cloneInitialData: {
        value: !0
      },
      strings: {},
      name: {
        value: null,
        validator: b.Squarespace.AttrValidators.isNullOrString
      },
      dialog: {
        value: null,
        validator: function(c) {
          if (b.Lang.isNull(c) || c instanceof b.Squarespace.EditingDialog || c.constructor instanceof b.Squarespace.EditingDialog.constructor) return !0;
          console.warn(this.name + ": Not an EditingDialog");
          return !1
        }
      },
      readOnly: {
        value: !1,
        validator: b.Squarespace.AttrValidators.isBoolean
      },
      required: {
        value: !1,
        validator: b.Squarespace.AttrValidators.isBoolean
      },
      errors: {
        value: [],
        readOnly: !0,
        getter: "_getErrors",
        validator: b.Squarespace.AttrValidators.isArray
      },
      errorFlyoutAnchor: {
        value: null,
        readOnly: !0,
        validator: b.Squarespace.AttrValidators.isNullOrInstanceOf(b.Node)
      },
      errorFlyoutRenderTarget: {
        value: void 0
      },
      errorFlyoutAnimationTime: {
        value: 0.3
      },
      focusable: {
        value: !0,
        validator: b.Lang.isBoolean
      }
    }
  })
}, "1.0", {
  requires: "base json-stringify squarespace-animations squarespace-attr-validators squarespace-dialog-field-legacy-interface squarespace-flyout-error-message-template squarespace-node-flyout squarespace-util squarespace-widgets-data-widget widget-anim widget-child widget-parent".split(" ")
});
YUI.add("dd-drop-plugin", function(b, c) {
  var a = function(b) {
    b.node = b.host;
    a.superclass.constructor.apply(this, arguments)
  };
  a.NAME = "dd-drop-plugin";
  a.NS = "drop";
  b.extend(a, b.DD.Drop);
  b.namespace("Plugin");
  b.Plugin.Drop = a
}, "3.17.2", {
  requires: ["dd-drop"]
});
YUI.add("squarespace-mailcheck", function(b) {
  b.namespace("Squarespace.Plugin");
  b.Squarespace.Plugin.MailCheck = b.Base.create("MailCheck", b.Plugin.Base, [], {
    initializer: function(b) {
      this._host = b.host;
      this._host.on("blur", this.checkMailAddress, this)
    },
    checkMailAddress: function(c) {
      c = this._host.get("value");
      null !== c && 3 < c.length && Kicksend.mailcheck.run({
        email: c,
        suggested: b.bind(this.emailSuggestions, this),
        empty: b.bind(this.noEmailSuggestions, this)
      })
    },
    emailSuggestions: function(c) {
      this._host.hasPlugin("flyoutPlugin") || this._host.plug(b.Squarespace.Animations.Flyout, {
        duration: 0.3
      });
      this.get("field").showError('Did you mean <a class="corrected-email" href="#">' + c.full + "</a>?");
      if (b.one(".corrected-email")) b.one(".corrected-email").once("click", this._onClick, this)
    },
    noEmailSuggestions: function(b) {
      (b = this._host.flyoutPlugin) && b.hide()
    },
    _onClick: function(b) {
      b.preventDefault();
      b.halt();
      if (b = this._host.flyoutPlugin) this._host.set("value", b.get("node").one("a").getHTML()), b.hide(), this.get("field").clearError()
    },
    destructor: function() {}
  }, {
    NS: "mailCheck",
    ATTRS: {
      field: {}
    }
  })
}, "1.0", {
  requires: ["thirdparty-kicksend", "squarespace-node-flyout"]
});
YUI.add("squarespace-gizmo", function(b) {
  b.namespace("Squarespace");
  var c = RegExp("[ ]+", "g"),
    a = RegExp("[^a-zA-Z0-9\\-]", "g"),
    d = function(a, d, e, c, l) {
      return (l ? b.Array.reject : b.Array.filter)(a, function(a) {
        for (var b = 0; b < e.length; ++b) if (a[e[b]] !== c[b]) return !1;
        d && d(a);
        return !0
      })
    }, e = function(a, d, c, k, l) {
      if (!this._destroyed) {
        if (b.Lang.isArray(d)) return b.Array.map(d, function(d) {
          return e.apply(this, [a].concat(b.Array(d)))
        }, this);
        if (b.Lang.isObject(d) && b.Lang.isFunction(d[a])) {
          var m = [c, k, l || this],
            m = m.concat(Array.prototype.slice.call(arguments,
            5)),
            m = d[a].apply(d, m);
          this._eventSubList.push({
            object: d,
            event: c,
            eventSub: m
          });
          return m
        }
        throw "Gizmo[" + this._name + "]: Could not subscribe to event: " + c;
      }
    };
  b.Squarespace.Gizmo = Class.create({
    _name: "Gizmo",
    _events: {
      render: {},
      rendered: {},
      destroy: {},
      destroyed: {}
    },
    initialize: function(a) {
      b.augment(this, b.EventTarget, !0, null, {
        prefix: this._name
      });
      this.params = b.merge(a);
      this._initState();
      a = this;
      for (var d = function(a, b) {
        this.publish(b, a)
      }; null !== a && void 0 !== a;) b.Object.each(a._events, d, this), a = a.superclass
    },
    getClassNames: function() {
      for (var a = this, b = [];; a = a.superclass) {
        if (!a) return b;
        b.push(a.getClassName())
      }
    },
    _initState: function() {
      this._parentEl = this._el = null;
      this._eventSubList = [];
      this._anims = [];
      this._timers = [];
      this.destroyed = this._destroyed = !1;
      this._children = [];
      this._guid = b.guid();
      this._name || (this._name = "No Name")
    },
    getClassName: function() {
      return this._name
    },
    getId: function() {
      return this._guid
    },
    getCssClassName: function() {
      var b = this._name,
        b = b.trim().replace(c, "-").replace(a, "").toLowerCase();
      return "squarespace-" + b
    },
    getElement: function() {
      return this._el
    },
    render: function(a) {
      this.fire("render");
      a = a || this._parentEl;
      this.params.noBoundingBox ? this._el = this._render() : (this._el = b.Node.create("<div></div>"), this._el.addClass(this.getCssClassName() + "-bbox"), this._el.append(this._render()));
      a && b.Lang.isFunction(a.append) && a.append(this._el);
      this._parentEl = a;
      this.fire("rendered")
    },
    _render: function() {},
    _subscribe: function(a, d, c, k) {
      var l = ["on"].concat(b.Array(arguments));
      return e.apply(this, l)
    },
    _subscribeOnce: function(a, d, c, k) {
      var l = ["once"].concat(b.Array(arguments));
      return e.apply(this, l)
    },
    _subscribeBefore: function(a, d, c, k) {
      var l = ["before"].concat(b.Array(arguments));
      return e.apply(this, l)
    },
    _subscribeAfter: function(a, d, c, k) {
      var l = ["after"].concat(b.Array(arguments));
      return e.apply(this, l)
    },
    _unsubscribe: function(a, e) {
      this._eventSubList = d(this._eventSubList, function(a) {
        a.eventSub.detach()
      }, e ? ["object", "event"] : ["object"], b.Array(arguments), !0)
    },
    _detach: function(a) {
      this._eventSubList = d(this._eventSubList, function(a) {
        a.eventSub.detach()
      }, ["eventSub"], [a], !0)
    },
    _clearEvents: function() {
      for (var a = this._eventSubList, b = 0; b < a.length; ++b) a[b].eventSub.detach();
      this._eventSubList = []
    },
    destroy: function() {
      if (this._destroyed) console.warn("Gizmo[" + this._name + "] already destroyed.");
      else {
        if (!this._eventSubList) throw console.error("Gizmo not initialized for...", this), "Gizmo[" + this._name + "] was never initialzed.  Missing _super?";
        this.fire("destroy");
        this._destroy();
        var a = 0;
        this._clearEvents();
        this._eventSubList = [];
        var d = [];
        b.Array.each(this._anims, function(a) {
          d.push(a)
        }, this);
        b.Array.each(d, function(a) {
          a.get("running") && a.stop(!1);
          a.destroy()
        });
        this._anims = null;
        for (a = this._children; 0 < a.length;) {
          var e = a[0];
          e._removeParent();
          e.destroy()
        }
        this._children = null;
        e = this._timers;
        for (a = 0; a < e.length; ++a) e[a].cancel();
        this._timers = null;
        this._el && b.Lang.isFunction(this._el.remove) && (this._el.remove(), this._el = null);
        this.destroyed = this._destroyed = !0;
        this.fire("destroyed")
      }
    },
    _destroy: function() {},
    isDestroyed: function() {
      return this._destroyed
    },
    _setParent: function(a) {
      this._parentGizmo = a
    },
    _removeParent: function() {
      this._parentGizmo && (this._parentGizmo._removeChild(this), this._parentGizmo = null)
    },
    _addChild: function(a) {
      this._children && (a._setParent && a._setParent(this), this._children.push(a))
    },
    _removeChild: function(a) {
      this._children.splice(this._children.indexOf(a), 1)
    },
    _getChildren: function() {
      return this._children
    },
    _anim: function(a) {
      if (!this._destroyed) {
        if (!a.node) throw "Gizmo[" + this._name + "]: Animation must specify a node!";
        var d;
        a.node.ancestor("body", !0) ? a.node._node ? (d = new b.Anim(a), d.on("end", function(a) {
          this._removeAnim(d)
        },
        this), this._anims.push(d)) : (console.warn("Gizmo[" + this._name + "]: _anim passed a YUI node with _node = null! Returning an empty animation."), console.trace(), d = new b.Anim) : (console.warn("Gizmo[" + this._name + "]: _anim passed a YUI node not in the DOM! Returning an empty animation."), d = new b.Anim);
        return d
      }
    },
    _removeAnim: function(a) {
      a = this._anims.indexOf(a); - 1 !== a && this._anims.splice(a, 1)
    },
    _trace: function(a) {
      for (var d = this.getEvent(a).getSubs(), e = [], c = function(a, b) {
        if (a.context && a.context.getName) e.push(a.context.getName());
        else {
          var d = {};
          d[a.fn.name] = a.fn.toString();
          e.push(d)
        }
      }, l = 0; l < d.length; ++l) b.Object.each(d[l], c);
      console.log("[trace] Event", a, "is notifying the following:", e);
      this.fire(a)
    },
    _later: function(a, d, e, c, l) {
      if (!this._destroyed) return a = b.later(a, e || this, d, c, l), this._timers.push(a), a
    },
    _cb: function(a) {
      return b.bind(function() {
        if (!this._destroyed) return a.apply(this, arguments)
      }, this)
    }
  });
  b.augment(b.Squarespace.Gizmo, b.EventTarget);
  b.Squarespace.ZombieGizmo = Class.extend(b.Squarespace.Gizmo, {
    _name: "ZombieGizmo",
    _events: {
      resurrect: {},
      resurrected: {}
    },
    initialize: function(a) {
      this._super(a)
    },
    destroy: function() {
      this._super();
      this.resurrect()
    },
    resurrect: function() {
      this.fire("resurrect");
      this._initState();
      this.fire("resurrected")
    }
  })
}, "1.0", {
  requires: ["array-extras", "node", "event-custom"]
});
YUI.add("dd-drag", function(b, c) {
  var a = b.DD.DDM,
    d = function(e) {
      this._lazyAddAttrs = !1;
      d.superclass.constructor.apply(this, arguments);
      a._regDrag(this) || b.error("Failed to register node, already in use: " + e.node)
    };
  d.NAME = "drag";
  d.START_EVENT = "mousedown";
  d.ATTRS = {
    node: {
      setter: function(a) {
        if (this._canDrag(a)) return a;
        var d = b.one(a);
        d || b.error("DD.Drag: Invalid Node Given: " + a);
        return d
      }
    },
    dragNode: {
      setter: function(a) {
        if (this._canDrag(a)) return a;
        var d = b.one(a);
        d || b.error("DD.Drag: Invalid dragNode Given: " + a);
        return d
      }
    },
    offsetNode: {
      value: !0
    },
    startCentered: {
      value: !1
    },
    clickPixelThresh: {
      value: a.get("clickPixelThresh")
    },
    clickTimeThresh: {
      value: a.get("clickTimeThresh")
    },
    lock: {
      value: !1,
      setter: function(b) {
        b ? this.get("node").addClass(a.CSS_PREFIX + "-locked") : this.get("node").removeClass(a.CSS_PREFIX + "-locked");
        return b
      }
    },
    data: {
      value: !1
    },
    move: {
      value: !0
    },
    useShim: {
      value: !0
    },
    activeHandle: {
      value: !1
    },
    primaryButtonOnly: {
      value: !0
    },
    dragging: {
      value: !1
    },
    parent: {
      value: !1
    },
    target: {
      value: !1,
      setter: function(a) {
        this._handleTarget(a);
        return a
      }
    },
    dragMode: {
      value: null,
      setter: function(b) {
        return a._setDragMode(b)
      }
    },
    groups: {
      value: ["default"],
      getter: function() {
        return !this._groups ? (this._groups = {}, []) : b.Object.keys(this._groups)
      },
      setter: function(a) {
        this._groups = b.Array.hash(a);
        return a
      }
    },
    handles: {
      value: null,
      setter: function(a) {
        a ? (this._handles = {}, b.Array.each(a, function(a) {
          var d = a;
          if (a instanceof b.Node || a instanceof b.NodeList) d = a._yuid;
          this._handles[d] = a
        }, this)) : this._handles = null;
        return a
      }
    },
    bubbles: {
      setter: function(a) {
        this.addTarget(a);
        return a
      }
    },
    haltDown: {
      value: !0
    }
  };
  b.extend(d, b.Base, {
    _canDrag: function(a) {
      return a && a.setXY && a.getXY && a.test && a.contains ? !0 : !1
    },
    _bubbleTargets: b.DD.DDM,
    addToGroup: function(b) {
      this._groups[b] = !0;
      a._activateTargets();
      return this
    },
    removeFromGroup: function(b) {
      delete this._groups[b];
      a._activateTargets();
      return this
    },
    target: null,
    _handleTarget: function(d) {
      b.DD.Drop && (!1 === d ? this.target && (a._unregTarget(this.target), this.target = null) : (b.Lang.isObject(d) || (d = {}), d.bubbleTargets = d.bubbleTargets || this.getTargets(),
      d.node = this.get("node"), d.groups = d.groups || this.get("groups"), this.target = new b.DD.Drop(d)))
    },
    _groups: null,
    _createEvents: function() {
      this.publish("drag:mouseDown", {
        defaultFn: this._defMouseDownFn,
        queuable: !1,
        emitFacade: !0,
        bubbles: !0,
        prefix: "drag"
      });
      this.publish("drag:align", {
        defaultFn: this._defAlignFn,
        queuable: !1,
        emitFacade: !0,
        bubbles: !0,
        prefix: "drag"
      });
      this.publish("drag:drag", {
        defaultFn: this._defDragFn,
        queuable: !1,
        emitFacade: !0,
        bubbles: !0,
        prefix: "drag"
      });
      this.publish("drag:end", {
        defaultFn: this._defEndFn,
        preventedFn: this._prevEndFn,
        queuable: !1,
        emitFacade: !0,
        bubbles: !0,
        prefix: "drag"
      });
      b.Array.each("drag:afterMouseDown drag:removeHandle drag:addHandle drag:removeInvalid drag:addInvalid drag:start drag:drophit drag:dropmiss drag:over drag:enter drag:exit".split(" "), function(a) {
        this.publish(a, {
          type: a,
          emitFacade: !0,
          bubbles: !0,
          preventable: !1,
          queuable: !1,
          prefix: "drag"
        })
      }, this)
    },
    _ev_md: null,
    _startTime: null,
    _endTime: null,
    _handles: null,
    _invalids: null,
    _invalidsDefault: {
      textarea: !0,
      input: !0,
      a: !0,
      button: !0,
      select: !0
    },
    _dragThreshMet: null,
    _fromTimeout: null,
    _clickTimeout: null,
    deltaXY: null,
    startXY: null,
    nodeXY: null,
    lastXY: null,
    actXY: null,
    realXY: null,
    mouseXY: null,
    region: null,
    _handleMouseUp: function() {
      this.fire("drag:mouseup");
      this._fixIEMouseUp();
      a.activeDrag && a._end()
    },
    _fixDragStart: function(a) {
      this.validClick(a) && a.preventDefault()
    },
    _ieSelectFix: function() {
      return !1
    },
    _ieSelectBack: null,
    _fixIEMouseDown: function() {
      b.UA.ie && (this._ieSelectBack = b.config.doc.body.onselectstart, b.config.doc.body.onselectstart = this._ieSelectFix)
    },
    _fixIEMouseUp: function() {
      b.UA.ie && (b.config.doc.body.onselectstart = this._ieSelectBack)
    },
    _handleMouseDownEvent: function(a) {
      this.validClick(a) && a.preventDefault();
      this.fire("drag:mouseDown", {
        ev: a
      })
    },
    _defMouseDownFn: function(e) {
      e = e.ev;
      this._dragThreshMet = !1;
      this._ev_md = e;
      if (this.get("primaryButtonOnly") && 1 < e.button) return !1;
      this.validClick(e) && (this._fixIEMouseDown(e), 0 !== d.START_EVENT.indexOf("gesture") && (this.get("haltDown") ? e.halt() : e.preventDefault()), this._setStartPosition([e.pageX, e.pageY]), a.activeDrag = this, this._clickTimeout = b.later(this.get("clickTimeThresh"), this, this._timeoutCheck));
      this.fire("drag:afterMouseDown", {
        ev: e
      })
    },
    validClick: function(a) {
      var d = !1,
        c = !1,
        h = a.target,
        k = null,
        l = c = null,
        m = !1;
      if (this._handles) b.Object.each(this._handles, function(a, e) {
        a instanceof b.Node || a instanceof b.NodeList ? d || (l = a, l instanceof b.Node && (l = new b.NodeList(a._node)), l.each(function(a) {
          a.contains(h) && (d = !0)
        })) : b.Lang.isString(e) && (h.test(e + ", " + e + " *") && !k) && (k = e, d = !0)
      });
      else if (c = this.get("node"), c.contains(h) || c.compareTo(h)) d = !0;
      d && this._invalids && b.Object.each(this._invalids, function(a, e) {
        b.Lang.isString(e) && h.test(e + ", " + e + " *") && (d = !1)
      });
      d && (k ? (c = a.currentTarget.all(k), m = !1, c.each(function(a) {
        if ((a.contains(h) || a.compareTo(h)) && !m) m = !0, this.set("activeHandle", a)
      }, this)) : this.set("activeHandle", this.get("node")));
      return d
    },
    _setStartPosition: function(a) {
      this.startXY = a;
      this.nodeXY = this.lastXY = this.realXY = this.get("node").getXY();
      this.get("offsetNode") ? this.deltaXY = [this.startXY[0] - this.nodeXY[0], this.startXY[1] -
        this.nodeXY[1]] : this.deltaXY = [0, 0]
    },
    _timeoutCheck: function() {
      !this.get("lock") && (!this._dragThreshMet && this._ev_md) && (this._fromTimeout = this._dragThreshMet = !0, this.start(), this._alignNode([this._ev_md.pageX, this._ev_md.pageY], !0))
    },
    removeHandle: function(a) {
      var d = a;
      if (a instanceof b.Node || a instanceof b.NodeList) d = a._yuid;
      this._handles[d] && (delete this._handles[d], this.fire("drag:removeHandle", {
        handle: a
      }));
      return this
    },
    addHandle: function(a) {
      this._handles || (this._handles = {});
      var d = a;
      if (a instanceof
      b.Node || a instanceof b.NodeList) d = a._yuid;
      this._handles[d] = a;
      this.fire("drag:addHandle", {
        handle: a
      });
      return this
    },
    removeInvalid: function(a) {
      this._invalids[a] && (this._invalids[a] = null, delete this._invalids[a], this.fire("drag:removeInvalid", {
        handle: a
      }));
      return this
    },
    addInvalid: function(a) {
      b.Lang.isString(a) && (this._invalids[a] = !0, this.fire("drag:addInvalid", {
        handle: a
      }));
      return this
    },
    initializer: function() {
      this.get("node").dd = this;
      if (!this.get("node").get("id")) {
        var a = b.stamp(this.get("node"));
        this.get("node").set("id",
        a)
      }
      this.actXY = [];
      this._invalids = b.clone(this._invalidsDefault, !0);
      this._createEvents();
      this.get("dragNode") || this.set("dragNode", this.get("node"));
      this.on("initializedChange", b.bind(this._prep, this));
      this.set("groups", this.get("groups"))
    },
    _prep: function() {
      this._dragThreshMet = !1;
      var e = this.get("node");
      e.addClass(a.CSS_PREFIX + "-draggable");
      e.on(d.START_EVENT, b.bind(this._handleMouseDownEvent, this));
      e.on("mouseup", b.bind(this._handleMouseUp, this));
      e.on("dragstart", b.bind(this._fixDragStart, this))
    },
    _unprep: function() {
      var b = this.get("node");
      b.removeClass(a.CSS_PREFIX + "-draggable");
      b.detachAll("mouseup");
      b.detachAll("dragstart");
      b.detachAll(d.START_EVENT);
      this.mouseXY = [];
      this.deltaXY = [0, 0];
      this.startXY = [];
      this.nodeXY = [];
      this.lastXY = [];
      this.actXY = [];
      this.realXY = []
    },
    start: function() {
      if (!this.get("lock") && !this.get("dragging")) {
        var b = this.get("node"),
          d, c;
        this._startTime = (new Date).getTime();
        a._start();
        b.addClass(a.CSS_PREFIX + "-dragging");
        this.fire("drag:start", {
          pageX: this.nodeXY[0],
          pageY: this.nodeXY[1],
          startTime: this._startTime
        });
        b = this.get("dragNode");
        c = this.nodeXY;
        d = b.get("offsetWidth");
        b = b.get("offsetHeight");
        this.get("startCentered") && this._setStartPosition([c[0] + d / 2, c[1] + b / 2]);
        this.region = {
          0: c[0],
          1: c[1],
          area: 0,
          top: c[1],
          right: c[0] + d,
          bottom: c[1] + b,
          left: c[0]
        };
        this.set("dragging", !0)
      }
      return this
    },
    end: function() {
      this._endTime = (new Date).getTime();
      this._clickTimeout && this._clickTimeout.cancel();
      this._dragThreshMet = this._fromTimeout = !1;
      !this.get("lock") && this.get("dragging") && this.fire("drag:end", {
        pageX: this.lastXY[0],
        pageY: this.lastXY[1],
        startTime: this._startTime,
        endTime: this._endTime
      });
      this.get("node").removeClass(a.CSS_PREFIX + "-dragging");
      this.set("dragging", !1);
      this.deltaXY = [0, 0];
      return this
    },
    _defEndFn: function() {
      this._fixIEMouseUp();
      this._ev_md = null
    },
    _prevEndFn: function() {
      this._fixIEMouseUp();
      this.get("dragNode").setXY(this.nodeXY);
      this.region = this._ev_md = null
    },
    _align: function(a) {
      this.fire("drag:align", {
        pageX: a[0],
        pageY: a[1]
      })
    },
    _defAlignFn: function(a) {
      this.actXY = [a.pageX - this.deltaXY[0], a.pageY - this.deltaXY[1]]
    },
    _alignNode: function(a,
    b) {
      this._align(a);
      b || this._moveNode()
    },
    _moveNode: function(a) {
      var b = [],
        d = [],
        c = this.nodeXY,
        k = this.actXY;
      b[0] = k[0] - this.lastXY[0];
      b[1] = k[1] - this.lastXY[1];
      d[0] = k[0] - this.nodeXY[0];
      d[1] = k[1] - this.nodeXY[1];
      this.region = {
        0: k[0],
        1: k[1],
        area: 0,
        top: k[1],
        right: k[0] + this.get("dragNode").get("offsetWidth"),
        bottom: k[1] + this.get("dragNode").get("offsetHeight"),
        left: k[0]
      };
      this.fire("drag:drag", {
        pageX: k[0],
        pageY: k[1],
        scroll: a,
        info: {
          start: c,
          xy: k,
          delta: b,
          offset: d
        }
      });
      this.lastXY = k
    },
    _defDragFn: function(a) {
      if (this.get("move")) {
        if (a.scroll && a.scroll.node) {
          var d = a.scroll.node.getDOMNode();
          d === b.config.win ? d.scrollTo(a.scroll.left, a.scroll.top) : (a.scroll.node.set("scrollTop", a.scroll.top), a.scroll.node.set("scrollLeft", a.scroll.left))
        }
        this.get("dragNode").setXY([a.pageX, a.pageY]);
        this.realXY = [a.pageX, a.pageY]
      }
    },
    _move: function(a) {
      if (this.get("lock")) return !1;
      this.mouseXY = [a.pageX, a.pageY];
      if (this._dragThreshMet) this._clickTimeout && this._clickTimeout.cancel(), this._alignNode([a.pageX, a.pageY]);
      else {
        var b = Math.abs(this.startXY[0] - a.pageX),
          d = Math.abs(this.startXY[1] - a.pageY);
        if (b > this.get("clickPixelThresh") || d > this.get("clickPixelThresh")) this._dragThreshMet = !0, this.start(), a && a.preventDefault && a.preventDefault(), this._alignNode([a.pageX, a.pageY])
      }
    },
    stopDrag: function() {
      this.get("dragging") && a._end();
      return this
    },
    destructor: function() {
      this._unprep();
      this.target && this.target.destroy();
      a._unregDrag(this)
    }
  });
  b.namespace("DD");
  b.DD.Drag = d
}, "3.17.2", {
  requires: ["dd-ddm-base"]
});
YUI.add("dd-proxy", function(b, c) {
  var a = b.DD.DDM,
    d = function() {
      d.superclass.constructor.apply(this, arguments)
    };
  d.NAME = "DDProxy";
  d.NS = "proxy";
  d.ATTRS = {
    host: {},
    moveOnEnd: {
      value: !0
    },
    hideOnEnd: {
      value: !0
    },
    resizeFrame: {
      value: !0
    },
    positionProxy: {
      value: !0
    },
    borderStyle: {
      value: "1px solid #808080"
    },
    cloneNode: {
      value: !1
    }
  };
  b.namespace("Plugin");
  b.extend(d, b.Base, {
    _hands: null,
    _init: function() {
      if (a._proxy) {
        this._hands || (this._hands = []);
        var d, c, g = this.get("host");
        g.get("dragNode").compareTo(g.get("node")) && a._proxy && g.set("dragNode", a._proxy);
        b.Array.each(this._hands, function(a) {
          a.detach()
        });
        d = a.on("ddm:start", b.bind(function() {
          a.activeDrag === g && a._setFrame(g)
        }, this));
        c = a.on("ddm:end", b.bind(function() {
          g.get("dragging") && (this.get("moveOnEnd") && g.get("node").setXY(g.lastXY), this.get("hideOnEnd") && g.get("dragNode").setStyle("display", "none"), this.get("cloneNode") && (g.get("dragNode").remove(), g.set("dragNode", a._proxy)))
        }, this));
        this._hands = [d, c]
      } else a._createFrame(), b.on("domready", b.bind(this._init, this))
    },
    initializer: function() {
      this._init()
    },
    destructor: function() {
      var a = this.get("host");
      b.Array.each(this._hands, function(a) {
        a.detach()
      });
      a.set("dragNode", a.get("node"))
    },
    clone: function() {
      var a = this.get("host"),
        d = a.get("node"),
        c = d.cloneNode(!0);
      c.all('input[type="radio"]').removeAttribute("name");
      delete c._yuid;
      c.setAttribute("id", b.guid());
      c.setStyle("position", "absolute");
      d.get("parentNode").appendChild(c);
      a.set("dragNode", c);
      return c
    }
  });
  b.Plugin.DDProxy = d;
  b.mix(a, {
    _createFrame: function() {
      if (!a._proxy) {
        a._proxy = !0;
        var d = b.Node.create("<div></div>"),
          c = b.one("body");
        d.setStyles({
          position: "absolute",
          display: "none",
          zIndex: "999",
          top: "-999px",
          left: "-999px"
        });
        c.prepend(d);
        d.set("id", b.guid());
        d.addClass(a.CSS_PREFIX + "-proxy");
        a._proxy = d
      }
    },
    _setFrame: function(b) {
      var d = b.get("node"),
        c = b.get("dragNode"),
        h, k = "auto";
      (h = a.activeDrag.get("activeHandle")) && (k = h.getStyle("cursor"));
      "auto" === k && (k = a.get("dragCursor"));
      c.setStyles({
        visibility: "hidden",
        display: "block",
        cursor: k,
        border: b.proxy.get("borderStyle")
      });
      b.proxy.get("cloneNode") && (c = b.proxy.clone());
      b.proxy.get("resizeFrame") && c.setStyles({
        height: d.get("offsetHeight") + "px",
        width: d.get("offsetWidth") + "px"
      });
      b.proxy.get("positionProxy") && c.setXY(b.nodeXY);
      c.setStyle("visibility", "visible")
    }
  })
}, "3.17.2", {
  requires: ["dd-drag"]
});
YUI.add("datatable-head", function(b, c) {
  var a = b.Lang,
    d = a.sub,
    e = a.isArray,
    f = b.Array;
  b.namespace("DataTable").HeaderView = b.Base.create("tableHeader", b.View, [], {
    CELL_TEMPLATE: '<th id="{id}" colspan="{_colspan}" rowspan="{_rowspan}" class="{className}" scope="col" {_id}{abbr}{title}>{content}</th>',
    ROW_TEMPLATE: "<tr>{content}</tr>",
    THEAD_TEMPLATE: '<thead class="{className}"></thead>',
    getClassName: function() {
      var a = this.host,
        d = a && a.constructor.NAME || this.constructor.NAME;
      return a && a.getClassName ? a.getClassName.apply(a,
      arguments) : b.ClassNameManager.getClassName.apply(b.ClassNameManager, [d].concat(f(arguments, 0, !0)))
    },
    render: function() {
      var a = this.get("container"),
        c = this.theadNode || (this.theadNode = this._createTHeadNode()),
        e = this.columns,
        f = {
          _colspan: 1,
          _rowspan: 1,
          abbr: "",
          title: ""
        }, m, n, p, s, r, q, u, t;
      if (c && e) {
        q = "";
        if (e.length) {
          m = 0;
          for (n = e.length; m < n; ++m) {
            u = "";
            p = 0;
            for (s = e[m].length; p < s; ++p) r = e[m][p], t = b.merge(f, r, {
              className: this.getClassName("header"),
              content: r.label || r.key || "Column " + (p + 1)
            }), t._id = r._id ? ' data-yui3-col-id="' + r._id + '"' : "", r.abbr && (t.abbr = ' abbr="' + r.abbr + '"'), r.title && (t.title = ' title="' + r.title + '"'), r.className && (t.className += " " + r.className), r._first && (t.className += " " + this.getClassName("first", "header")), r._id && (t.className += " " + this.getClassName("col", r._id)), u += d(r.headerTemplate || this.CELL_TEMPLATE, t);
            q += d(this.ROW_TEMPLATE, {
              content: u
            })
          }
        }
        c.setHTML(q);
        c.get("parentNode") !== a && a.insertBefore(c, a.one("tfoot, tbody"))
      }
      this.bindUI();
      return this
    },
    _afterColumnsChange: function(a) {
      this.columns = this._parseColumns(a.newVal);
      this.render()
    },
    bindUI: function() {
      this._eventHandles.columnsChange || (this._eventHandles.columnsChange = this.after("columnsChange", b.bind("_afterColumnsChange", this)))
    },
    _createTHeadNode: function() {
      return b.Node.create(d(this.THEAD_TEMPLATE, {
        className: this.getClassName("columns")
      }))
    },
    destructor: function() {
      (new b.EventHandle(b.Object.values(this._eventHandles))).detach()
    },
    initializer: function(a) {
      this.host = a.host;
      this.columns = this._parseColumns(a.columns);
      this._eventHandles = []
    },
    _parseColumns: function(a) {
      var d = [],
        c = [],
        f = 1,
        m, n, p, s, r, q, u;
      if (e(a) && a.length) {
        a = a.slice();
        for (c.push([a, - 1]); c.length;) {
          m = c[c.length - 1];
          n = m[0];
          q = m[1] + 1;
          for (u = n.length; q < u; ++q) if (n[q] = p = b.merge(n[q]), s = p.children, b.stamp(p), p.id || (p.id = b.guid()), e(s) && s.length) {
            c.push([s, - 1]);
            m[1] = q;
            f = Math.max(f, c.length);
            break
          } else p._colspan = 1;
          if (q >= u) {
            if (1 < c.length) {
              m = c[c.length - 2];
              r = m[0][m[1]];
              q = r._colspan = 0;
              for (u = n.length; q < u; ++q) n[q]._parent = r, r._colspan += n[q]._colspan
            }
            c.pop()
          }
        }
        for (q = 0; q < f; ++q) d.push([]);
        for (c.push([a, - 1]); c.length;) {
          m = c[c.length - 1];
          n = m[0];
          q = m[1] + 1;
          for (u = n.length; q < u; ++q) {
            p = n[q];
            s = p.children;
            d[c.length - 1].push(p);
            m[1] = q;
            p._headers = [p.id];
            for (a = c.length - 2; 0 <= a; --a) r = c[a][0][c[a][1]], p._headers.unshift(r.id);
            if (s && s.length) {
              c.push([s, - 1]);
              break
            } else p._rowspan = f - c.length + 1
          }
          q >= u && c.pop()
        }
      }
      q = 0;
      for (u = d.length; q < u; q += p._rowspan) p = d[q][0], p._first = !0;
      return d
    }
  })
}, "3.17.2", {
  requires: ["datatable-core", "view", "classnamemanager"]
});
YUI.add("range-slider", function(b, c) {
  b.Slider = b.Base.build("slider", b.SliderBase, [b.SliderValueRange, b.ClickableRail])
}, "3.17.2", {
  requires: ["slider-base", "slider-value-range", "clickable-rail"]
});
YUI.add("datatable-mutable", function(b, c) {
  var a = b.Array,
    d = b.Lang,
    e = d.isString,
    f = d.isArray,
    g = d.isObject,
    h = d.isNumber,
    k = b.Array.indexOf,
    l;
  b.namespace("DataTable").Mutable = l = function() {};
  l.ATTRS = {
    autoSync: {
      value: !1,
      validator: d.isBoolean
    }
  };
  b.mix(l.prototype, {
    addColumn: function(a, b) {
      e(a) && (a = {
        key: a
      });
      if (a) {
        if (2 > arguments.length || !h(b) && !f(b)) b = this.get("columns").length;
        this.fire("addColumn", {
          column: a,
          index: b
        })
      }
      return this
    },
    modifyColumn: function(a, b) {
      e(b) && (b = {
        key: b
      });
      g(b) && this.fire("modifyColumn", {
        column: a,
        newColumnDef: b
      });
      return this
    },
    moveColumn: function(a, b) {
      void 0 !== a && (h(b) || f(b)) && this.fire("moveColumn", {
        column: a,
        index: b
      });
      return this
    },
    removeColumn: function(a) {
      void 0 !== a && this.fire("removeColumn", {
        column: a
      });
      return this
    },
    addRow: function(b, d) {
      var c = d && "sync" in d ? d.sync : this.get("autoSync"),
        e, f, g, h;
      if (b && this.data && (e = this.data.add.apply(this.data, arguments), c)) {
        e = a(e);
        h = a(arguments, 1, !0);
        f = 0;
        for (g = e.length; f < g; ++f) c = e[f], c.isNew() && e[f].save.apply(e[f], h)
      }
      return this
    },
    removeRow: function(b, d) {
      var c = this.data,
        e = d && "sync" in d ? d.sync : this.get("autoSync"),
        f, h, k;
      g(b) && b instanceof this.get("recordType") ? f = b : c && void 0 !== b && (f = c.getById(b) || c.getByClientId(b) || c.item(b));
      if (f && (k = a(arguments, 1, !0), c = c.remove.apply(c, [f].concat(k)), e)) {
        g(k[0]) || k.unshift({});
        k[0]["delete"] = !0;
        c = a(c);
        e = 0;
        for (h = c.length; e < h; ++e) f = c[e], f.destroy.apply(f, k)
      }
      return this
    },
    modifyRow: function(b, d, c) {
      var e = this.data,
        f = c && "sync" in c ? c.sync : this.get("autoSync"),
        h;
      g(b) && b instanceof this.get("recordType") ? h = b : e && void 0 !== b && (h = e.getById(b) || e.getByClientId(b) || e.item(b));
      h && g(d) && (e = a(arguments, 1, !0), h.setAttrs.apply(h, e), f && !h.isNew() && h.save.apply(h, e));
      return this
    },
    _defAddColumnFn: function(b) {
      var d = a(b.index),
        c = this.get("columns"),
        e = c,
        f, g;
      f = 0;
      for (g = d.length - 1; e && f < g; ++f) e = e[d[f]] && e[d[f]].children;
      e && (e.splice(d[f], 0, b.column), this.set("columns", c, {
        originEvent: b
      }))
    },
    _defModifyColumnFn: function(a) {
      var d = this.get("columns"),
        c = this.getColumn(a.column);
      c && (b.mix(c, a.newColumnDef, !0), this.set("columns", d, {
        originEvent: a
      }))
    },
    _defMoveColumnFn: function(b) {
      var d = this.get("columns"),
        c = this.getColumn(b.column),
        e = a(b.index),
        f, g, h, l, z;
      if (c && (f = c._parent ? c._parent.children : d, g = k(f, c), - 1 < g)) {
        h = d;
        l = 0;
        for (z = e.length - 1; h && l < z; ++l) h = h[e[l]] && h[e[l]].children;
        h && (z = h.length, f.splice(g, 1), e = e[l], z > h.lenth && g < e && e--, h.splice(e, 0, c), this.set("columns", d, {
          originEvent: b
        }))
      }
    },
    _defRemoveColumnFn: function(a) {
      var d = this.get("columns"),
        c = this.getColumn(a.column),
        e;
      c && (e = c._parent ? c._parent.children : d, c = b.Array.indexOf(e, c), - 1 < c && (e.splice(c,
      1), this.set("columns", d, {
        originEvent: a
      })))
    },
    initializer: function() {
      this.publish({
        addColumn: {
          defaultFn: b.bind("_defAddColumnFn", this)
        },
        removeColumn: {
          defaultFn: b.bind("_defRemoveColumnFn", this)
        },
        moveColumn: {
          defaultFn: b.bind("_defMoveColumnFn", this)
        },
        modifyColumn: {
          defaultFn: b.bind("_defModifyColumnFn", this)
        }
      })
    }
  });
  l.prototype.addRows = l.prototype.addRow;
  d.isFunction(b.DataTable) && b.Base.mix(b.DataTable, [l])
}, "3.17.2", {
  requires: ["datatable-base"]
});
var Kicksend = {
  mailcheck: {
    threshold: 3,
    defaultDomains: "yahoo.com google.com hotmail.com gmail.com me.com aol.com mac.com live.com comcast.net googlemail.com msn.com hotmail.co.uk yahoo.co.uk facebook.com verizon.net sbcglobal.net att.net gmx.com mail.com".split(" "),
    defaultTopLevelDomains: "co.uk com net org info edu gov mil au com.au se de me dk co.nz it be".split(" "),
    run: function(b) {
      b.domains = b.domains || Kicksend.mailcheck.defaultDomains;
      b.topLevelDomains = b.topLevelDomains || Kicksend.mailcheck.defaultTopLevelDomains;
      b.distanceFunction = b.distanceFunction || Kicksend.sift3Distance;
      var c = Kicksend.mailcheck.suggest(encodeURI(b.email), b.domains, b.topLevelDomains, b.distanceFunction);
      c ? b.suggested && b.suggested(c) : b.empty && b.empty()
    },
    suggest: function(b, c, a, d) {
      b = b.toLowerCase();
      b = this.splitEmail(b);
      if (c = this.findClosestDomain(b.domain, c, d)) {
        if (c != b.domain) return {
          address: b.address,
          domain: c,
          full: b.address + "@" + c
        }
      } else if (a = this.findClosestDomain(b.topLevelDomain, a), b.domain && a && a != b.topLevelDomain) return c = b.domain, c = c.substring(0,
      c.lastIndexOf(b.topLevelDomain)) + a, {
        address: b.address,
        domain: c,
        full: b.address + "@" + c
      };
      return !1
    },
    findClosestDomain: function(b, c, a) {
      var d, e = 99,
        f = null;
      if (!b || !c) return !1;
      a || (a = this.sift3Distance);
      for (var g = 0; g < c.length; g++) {
        if (b === c[g]) return b;
        d = a(b, c[g]);
        d < e && (e = d, f = c[g])
      }
      return e <= this.threshold && null !== f ? f : !1
    },
    sift3Distance: function(b, c) {
      if (null == b || 0 === b.length) return null == c || 0 === c.length ? 0 : c.length;
      if (null == c || 0 === c.length) return b.length;
      for (var a = 0, d = 0, e = 0, f = 0; a + d < b.length && a + e < c.length;) {
        if (b.charAt(a + d) == c.charAt(a + e)) f++;
        else for (var g = e = d = 0; 5 > g; g++) {
          if (a + g < b.length && b.charAt(a + g) == c.charAt(a)) {
            d = g;
            break
          }
          if (a + g < c.length && b.charAt(a) == c.charAt(a + g)) {
            e = g;
            break
          }
        }
        a++
      }
      return (b.length + c.length) / 2 - f
    },
    splitEmail: function(b) {
      b = b.split("@");
      if (2 > b.length) return !1;
      for (var c = 0; c < b.length; c++) if ("" === b[c]) return !1;
      var a = b.pop(),
        d = a.split("."),
        e = "";
      if (0 == d.length) return !1;
      if (1 == d.length) e = d[0];
      else {
        for (c = 1; c < d.length; c++) e += d[c] + ".";
        2 <= d.length && (e = e.substring(0, e.length - 1))
      }
      return {
        topLevelDomain: e,
        domain: a,
        address: b.join("@")
      }
    }
  }
};
window.jQuery && function(b) {
  b.fn.mailcheck = function(b) {
    var a = this;
    if (b.suggested) {
      var d = b.suggested;
      b.suggested = function(b) {
        d(a, b)
      }
    }
    if (b.empty) {
      var e = b.empty;
      b.empty = function() {
        e.call(null, a)
      }
    }
    b.email = this.val();
    Kicksend.mailcheck.run(b)
  }
}(jQuery);
YUI.add("thirdparty-kicksend", function(b) {}, "1.0", {});
YUI.add("squarespace-flyout-error-message-template", function(b) {
  var c = b.Handlebars;
  (function() {
    var a = c.template;
    (c.templates = c.templates || {})["flyout-error-message.html"] = a(function(a, b, c, g, h) {
      this.compilerInfo = [4, ">= 1.0.0"];
      c = this.merge(c, a.helpers);
      h = h || {};
      a = '<div class="sqs-flyout-error-message">';
      (c = c.message) ? c = c.call(b, {
        hash: {},
        data: h
      }) : (c = b.message, c = "function" === typeof c ? c.apply(b) : c);
      if (c || 0 === c) a += c;
      return a + "</div>"
    })
  })();
  b.Handlebars.registerPartial("flyout-error-message.html".replace("/", "."), c.templates["flyout-error-message.html"])
}, "1.0", {
  requires: ["handlebars-base"]
});
YUI.add("dd-delegate", function(b, c) {
  var a = function() {
    a.superclass.constructor.apply(this, arguments)
  }, d = b.Node.create("<div>Temp Node</div>");
  b.extend(a, b.Base, {
    _bubbleTargets: b.DD.DDM,
    dd: null,
    _shimState: null,
    _handles: null,
    _onNodeChange: function(a) {
      this.set("dragNode", a.newVal)
    },
    _afterDragEnd: function() {
      b.DD.DDM._noShim = this._shimState;
      this.set("lastNode", this.dd.get("node"));
      this.get("lastNode").removeClass(b.DD.DDM.CSS_PREFIX + "-dragging");
      this.dd._unprep();
      this.dd.set("node", d)
    },
    _delMouseDown: function(a) {
      var d = a.currentTarget,
        c = this.dd,
        h = d,
        k = this.get("dragConfig");
      d.test(this.get("nodes")) && !d.test(this.get("invalid")) && (this._shimState = b.DD.DDM._noShim, b.DD.DDM._noShim = !0, this.set("currentNode", d), c.set("node", d), k && k.dragNode ? h = k.dragNode : c.proxy && (h = b.DD.DDM._proxy), c.set("dragNode", h), c._prep(), c.fire("drag:mouseDown", {
        ev: a
      }))
    },
    _onMouseEnter: function() {
      this._shimState = b.DD.DDM._noShim;
      b.DD.DDM._noShim = !0
    },
    _onMouseLeave: function() {
      b.DD.DDM._noShim = this._shimState
    },
    initializer: function() {
      this._handles = [];
      var a = this.get("dragConfig") || {}, c = this.get("container");
      a.node = d.cloneNode(!0);
      a.bubbleTargets = this;
      this.get("handles") && (a.handles = this.get("handles"));
      this.dd = new b.DD.Drag(a);
      this.dd.after("drag:end", b.bind(this._afterDragEnd, this));
      this.dd.on("dragNodeChange", b.bind(this._onNodeChange, this));
      this.dd.after("drag:mouseup", function() {
        this._unprep()
      });
      this._handles.push(b.delegate(b.DD.Drag.START_EVENT, b.bind(this._delMouseDown, this), c, this.get("nodes")));
      this._handles.push(b.on("mouseenter",
      b.bind(this._onMouseEnter, this), c));
      this._handles.push(b.on("mouseleave", b.bind(this._onMouseLeave, this), c));
      b.later(50, this, this.syncTargets);
      b.DD.DDM.regDelegate(this)
    },
    syncTargets: function() {
      if (b.Plugin.Drop && !this.get("destroyed")) {
        var a, d, c;
        if (this.get("target")) {
          a = b.one(this.get("container")).all(this.get("nodes"));
          d = this.dd.get("groups");
          if ((c = this.get("dragConfig")) && c.groups) d = c.groups;
          a.each(function(a) {
            this.createDrop(a, d)
          }, this)
        }
        return this
      }
    },
    createDrop: function(a, d) {
      var c = {
        useShim: !1,
        bubbleTargets: this
      };
      a.drop || a.plug(b.Plugin.Drop, c);
      a.drop.set("groups", d);
      return a
    },
    destructor: function() {
      this.dd && this.dd.destroy();
      b.Plugin.Drop && b.one(this.get("container")).all(this.get("nodes")).unplug(b.Plugin.Drop);
      b.Array.each(this._handles, function(a) {
        a.detach()
      })
    }
  }, {
    NAME: "delegate",
    ATTRS: {
      container: {
        value: "body"
      },
      nodes: {
        value: ".dd-draggable"
      },
      invalid: {
        value: "input, select, button, a, textarea"
      },
      lastNode: {
        value: d
      },
      currentNode: {
        value: d
      },
      dragNode: {
        value: d
      },
      over: {
        value: !1
      },
      target: {
        value: !1
      },
      dragConfig: {
        value: null
      },
      handles: {
        value: null
      }
    }
  });
  b.mix(b.DD.DDM, {
    _delegates: [],
    regDelegate: function(a) {
      this._delegates.push(a)
    },
    getDelegate: function(a) {
      var d = null;
      a = b.one(a);
      b.Array.each(this._delegates, function(b) {
        a.test(b.get("container")) && (d = b)
      }, this);
      return d
    }
  });
  b.namespace("DD");
  b.DD.Delegate = a
}, "3.17.2", {
  requires: ["dd-drag", "dd-drop-plugin", "event-mouseenter"]
});
YUI.add("squarespace-checkout-shopping-cart-item-template", function(b) {
  var c = b.Handlebars;
  (function() {
    var a = c.template;
    (c.templates = c.templates || {})["checkout-shopping-cart-item.html"] = a(function(a, b, c, g, h) {
      this.compilerInfo = [4, ">= 1.0.0"];
      c = this.merge(c, a.helpers);
      h = h || {};
      a = '<tr>\n\n  <td class="item">\n    <div class="item-image"></div>\n    <div class="item-desc"></div>\n  </td>\n\n  <td class="quantity">\n    ';
      if ((b = c["if"].call(b, b.isPhysicalProduct, {
        hash: {},
        inverse: this.program(3, function(a,
        b) {
          return '\n      <div class="not-applicable">N/A</div>\n    '
        }, h),
        fn: this.program(1, function(a, b) {
          return "\n      <input />\n    "
        }, h),
        data: h
      })) || 0 === b) a += b;
      return a + '\n  </td>\n\n  <td class="price"></td>\n\n  <td class="remove">\n    <div class="remove-item"></div>\n  </td>\n\n</tr>\n'
    })
  })();
  b.Handlebars.registerPartial("checkout-shopping-cart-item.html".replace("/", "."), c.templates["checkout-shopping-cart-item.html"])
}, "1.0", {
  requires: ["handlebars-base"]
});
YUI.add("dd-ddm-drop", function(b, c) {
  b.mix(b.DD.DDM, {
    _noShim: !1,
    _activeShims: [],
    _hasActiveShim: function() {
      return this._noShim ? !0 : this._activeShims.length
    },
    _addActiveShim: function(a) {
      this._activeShims.push(a)
    },
    _removeActiveShim: function(a) {
      var d = [];
      b.Array.each(this._activeShims, function(b) {
        b._yuid !== a._yuid && d.push(b)
      });
      this._activeShims = d
    },
    syncActiveShims: function(a) {
      b.later(0, this, function(a) {
        a = a ? this.targets : this._lookup();
        b.Array.each(a, function(a) {
          a.sizeShim.call(a)
        }, this)
      }, a)
    },
    mode: 0,
    POINT: 0,
    INTERSECT: 1,
    STRICT: 2,
    useHash: !0,
    activeDrop: null,
    validDrops: [],
    otherDrops: {},
    targets: [],
    _addValid: function(a) {
      this.validDrops.push(a);
      return this
    },
    _removeValid: function(a) {
      var d = [];
      b.Array.each(this.validDrops, function(b) {
        b !== a && d.push(b)
      });
      this.validDrops = d;
      return this
    },
    isOverTarget: function(a) {
      if (this.activeDrag && a) {
        var b = this.activeDrag.mouseXY,
          c = this.activeDrag.get("dragMode"),
          f, g = a.shim;
        if (b && this.activeDrag) {
          f = this.activeDrag.region;
          if (c === this.STRICT) return this.activeDrag.get("dragNode").inRegion(a.region, !0, f);
          if (a && a.shim) {
            if (c === this.INTERSECT && this._noShim) return b = f || this.activeDrag.get("node"), a.get("node").intersect(b, a.region).inRegion;
            this._noShim && (g = a.get("node"));
            return g.intersect({
              top: b[1],
              bottom: b[1],
              left: b[0],
              right: b[0]
            }, a.region).inRegion
          }
        }
      }
      return !1
    },
    clearCache: function() {
      this.validDrops = [];
      this.otherDrops = {};
      this._activeShims = []
    },
    _activateTargets: function() {
      this._noShim = !0;
      this.clearCache();
      b.Array.each(this.targets, function(a) {
        a._activateShim([]);
        !0 === a.get("noShim") && (this._noShim = !1)
      }, this);
      this._handleTargetOver()
    },
    getBestMatch: function(a, d) {
      var c = null,
        f = 0,
        g;
      b.Object.each(a, function(a) {
        var b = this.activeDrag.get("dragNode").intersect(a.get("node"));
        a.region.area = b.area;
        b.inRegion && b.area > f && (f = b.area, c = a)
      }, this);
      return d ? (g = [], b.Object.each(a, function(a) {
        a !== c && g.push(a)
      }, this), [c, g]) : c
    },
    _deactivateTargets: function() {
      var a = [],
        d = this.activeDrag,
        c = this.activeDrop;
      d && c && this.otherDrops[c] ? (d.get("dragMode") ? (a = this.getBestMatch(this.otherDrops, !0), c = a[0], a = a[1]) : (a = this.otherDrops,
      delete a[c]), d.get("node").removeClass(this.CSS_PREFIX + "-drag-over"), c && (c.fire("drop:hit", {
        drag: d,
        drop: c,
        others: a
      }), d.fire("drag:drophit", {
        drag: d,
        drop: c,
        others: a
      }))) : d && d.get("dragging") && (d.get("node").removeClass(this.CSS_PREFIX + "-drag-over"), d.fire("drag:dropmiss", {
        pageX: d.lastXY[0],
        pageY: d.lastXY[1]
      }));
      this.activeDrop = null;
      b.Array.each(this.targets, function(a) {
        a._deactivateShim([])
      }, this)
    },
    _dropMove: function() {
      this._hasActiveShim() ? this._handleTargetOver() : b.Object.each(this.otherDrops, function(a) {
        a._handleOut.apply(a, [])
      })
    },
    _lookup: function() {
      if (!this.useHash || this._noShim) return this.validDrops;
      var a = [];
      b.Array.each(this.validDrops, function(b) {
        b.shim && b.shim.inViewportRegion(!1, b.region) && a.push(b)
      });
      return a
    },
    _handleTargetOver: function() {
      var a = this._lookup();
      b.Array.each(a, function(a) {
        a._handleTargetOver.call(a)
      }, this)
    },
    _regTarget: function(a) {
      this.targets.push(a)
    },
    _unregTarget: function(a) {
      var d = [],
        c;
      b.Array.each(this.targets, function(b) {
        b !== a && d.push(b)
      }, this);
      this.targets = d;
      c = [];
      b.Array.each(this.validDrops,

      function(b) {
        b !== a && c.push(b)
      });
      this.validDrops = c
    },
    getDrop: function(a) {
      var d = !1,
        c = b.one(a);
      c instanceof b.Node && b.Array.each(this.targets, function(a) {
        c.compareTo(a.get("node")) && (d = a)
      });
      return d
    }
  }, !0)
}, "3.17.2", {
  requires: ["dd-ddm"]
});
YUI.add("dd-ddm", function(b, c) {
  b.mix(b.DD.DDM, {
    _pg: null,
    _debugShim: !1,
    _activateTargets: function() {},
    _deactivateTargets: function() {},
    _startDrag: function() {
      this.activeDrag && this.activeDrag.get("useShim") && (this._shimming = !0, this._pg_activate(), this._activateTargets())
    },
    _endDrag: function() {
      this._pg_deactivate();
      this._deactivateTargets()
    },
    _pg_deactivate: function() {
      this._pg.setStyle("display", "none")
    },
    _pg_activate: function() {
      this._pg || this._createPG();
      var a = this.activeDrag.get("activeHandle"),
        b = "auto";
      a && (b = a.getStyle("cursor"));
      "auto" === b && (b = this.get("dragCursor"));
      this._pg_size();
      this._pg.setStyles({
        top: 0,
        left: 0,
        display: "block",
        opacity: this._debugShim ? ".5" : "0",
        cursor: b
      })
    },
    _pg_size: function() {
      if (this.activeDrag) {
        var a = b.one("body"),
          d = a.get("docHeight"),
          a = a.get("docWidth");
        this._pg.setStyles({
          height: d + "px",
          width: a + "px"
        })
      }
    },
    _createPG: function() {
      var a = b.Node.create("<div></div>"),
        d = b.one("body");
      a.setStyles({
        top: "0",
        left: "0",
        position: "absolute",
        zIndex: "9999",
        overflow: "hidden",
        backgroundColor: "red",
        display: "none",
        height: "5px",
        width: "5px"
      });
      a.set("id", b.stamp(a));
      a.addClass(b.DD.DDM.CSS_PREFIX + "-shim");
      d.prepend(a);
      this._pg = a;
      this._pg.on("mousemove", b.throttle(b.bind(this._move, this), this.get("throttleTime")));
      this._pg.on("mouseup", b.bind(this._end, this));
      a = b.one("win");
      b.on("window:resize", b.bind(this._pg_size, this));
      a.on("scroll", b.bind(this._pg_size, this))
    }
  }, !0)
}, "3.17.2", {
  requires: ["dd-ddm-base", "event-resize"]
});
YUI.add("squarespace-animation-manager", function(b) {
  b.namespace("Squarespace").AnimationManager = Class.create({
    initialize: function() {
      this._concurrentAnimations = [];
      this._destroyed = this._animationsFinished = !1
    },
    push: function(c) {
      if (b.Lang.isArray(c)) for (var a = 0; a < c.length; a++) this._concurrentAnimations.push(c[a]);
      else this._concurrentAnimations.push(c)
    },
    run: function(b, a) {
      this.callback = b;
      this.context = a || this;
      for (var d = function() {
        this._animEnd()
      }, e = 0; e < this._concurrentAnimations.length; e++) this._concurrentAnimations[e] && (this._concurrentAnimations[e].once("end", d, this), this._concurrentAnimations[e].run());
      this._running = !0
    },
    stop: function(b) {
      b = !! b;
      for (var a = 0; a < this._concurrentAnimations.length; a++) this._concurrentAnimations[a] && (this._concurrentAnimations[a].stop(b), this._animationsFinished = !0);
      this._running && (this._running = !1)
    },
    _animEnd: function() {
      b.Array.every(this._concurrentAnimations, function(b) {
        return !b.get("running")
      }) && (this._animationsFinished = !0, this._running = !1, this.callback && this.callback.call(this.context),
      this.fire("end"))
    },
    isRunning: function() {
      return this._running
    },
    destroy: function(b) {
      this.fire("destroy");
      if (this._destroyed) throw "MultipleAnimationManager: Sorry bro, I'm already destroyed.";
      this._destroyed = !0;
      this.stop(b);
      for (var a = 0; a < this._concurrentAnimations.length; a++) this._concurrentAnimations[a] && this._concurrentAnimations[a].destroy(!0);
      this._running && (this._running = !1, this._animationsFinished = !! b);
      this.fire("destroyed")
    },
    isDestroyed: function() {
      return this._destroyed
    }
  });
  b.augment(b.Squarespace.AnimationManager,
  b.EventTarget)
}, "1.0");
YUI.add("squarespace-checkout-form", function(b) {
  b.namespace("Squarespace.Widgets");
  var c = b.Squarespace.Widgets.CheckoutForm = b.Base.create("checkoutForm", b.Squarespace.Widgets.SSWidget, [], {
    initializer: function() {
      var a = this.get("model"),
        d = this.get("countriesAllowed");
      this._billingSection = new b.Squarespace.Widgets.CheckoutFormBilling({
        model: a,
        countriesAllowed: d,
        optionalFields: this.get("optionalFields"),
        useAddressForShipping: !0,
        enableMailingListOptInByDefault: this.get("enableMailingListOptInByDefault"),
        state: "editing"
      });
      this._shippingSection = new b.Squarespace.Widgets.CheckoutFormShipping({
        model: a,
        countriesAllowed: d,
        useAddressForShipping: !1,
        state: "incomplete"
      });
      this._shippingUpdateQueue = new b.AsyncQueue;
      a = this.get("storeCurrencyCode");
      d = "Submit Order";["EUR", "GBP"].contains(a) && (d = "Order & Pay Now");
      this._paymentSection = new b.Squarespace.Widgets.CheckoutFormPayment({
        state: "incomplete",
        "strings.submitText": d,
        inTestMode: this.get("inTestMode")
      });
      if (a = this.get("optionalFields.customForm")) this._customFormSection = new b.Squarespace.Widgets.CheckoutCustomForm({
        state: "incomplete",
        customForm: a,
        formId: a.id,
        "strings.name": a.name
      });
      this._sectionsInFlow = this._getOrderedSectionList();
      this._allSections = [this._billingSection, this._shippingSection, this._customFormSection, this._paymentSection]
    },
    destructor: function() {
      b.Array.invoke(this._allSections, "destroy");
      this._shippingUpdateQueue.destroy();
      this._shippingUpdateQueue = null
    },
    renderUI: function() {
      c.superclass.renderUI.call(this);
      var a;
      a = b.Data.addCrumb("/commerce/submit-order");
      a = b.Node.create('<form action="' + a + '" method="POST" accept-charset="UTF-8" enctype="application/x-www-form-urlencoded; charset=utf-8"></form>');
      this._forceTestModeInput = a.appendChild('<input type="hidden" name="forceTestMode">');
      a.append(b.Node.create('<input type="hidden" name="customFormSubmission" />'));
      b.Array.invoke(this._allSections, "render", a);
      this._collapseAllButOne(this._billingSection);
      this.get("contentBox").append(a)
    },
    bindUI: function() {
      c.superclass.bindUI.call(this);
      this._shippingSection.on("shippingLocationChange", this._updateCartShippingLocationFromShipping, this);
      this._billingSection.on("shippingLocationChange", this._updateCartShippingLocationFromBilling,
      this);
      this.get("model").on("change", this.syncUI, this);
      this.after("inTestModeChange", this.syncUI, this);
      this._bindFlowControl();
      this._bindSectionDataDependencies()
    },
    syncUI: function() {
      if (!this.get("model").get("requiresShipping")) {
        var a = this._shippingSection;
        switch (a.get("state")) {
        case "editing":
          a.setStateComplete();
          this._paymentSection.setStateEditing();
          break;
        case "complete":
          a.setStateComplete()
        }
      }
      a = this.get("inTestMode");
      this._paymentSection.set("inTestMode", a)
    },
    lock: function() {
      this.fire("lock");
      this.get("contentBox").addClass("submitting");
      this._paymentSection.lock()
    },
    unlock: function() {
      this.fire("unlock");
      this.get("contentBox").removeClass("submitting");
      this._paymentSection.unlock()
    },
    _bindSectionDataDependencies: function() {
      var a = this._billingSection,
        b = this._shippingSection,
        c = this._paymentSection;
      a.on("continue", function() {
        var b = a.getValues();
        c.setValues({
          cardHolderName: b.billingFirstName + " " + b.billingLastName
        });
        a.get("useAddressForShipping") && this._updateCartShippingLocationFromBilling()
      }, this);
      b.on("continue", function() {
        a.get("useAddressForShipping") || this._updateCartShippingLocationFromShipping()
      }, this);
      b.on("useAddressForShippingChange", function(b) {
        a.set("useAddressForShipping", !b.newVal)
      }, this)
    },
    _bindFlowControl: function() {
      b.Array.each(this._sectionsInFlow, function(a, b) {
        if (b === this._sectionsInFlow.length - 1) a.on("continue", this._submit, this);
        else a.on("continue", function() {
          this._collapseAllButOne(this._sectionsInFlow[b + 1])
        }, this);
        a.on("edit", function() {
          this._collapseAllButOne(a)
        }, this)
      }, this)
    },
    _collapseAllButOne: function(a) {
      var d = this._sectionsInFlow.indexOf(a);
      b.Array.each(this._sectionsInFlow, function(a, b) {
        b === d ? a.setStateEditing() : b < d ? a.setStateComplete() : a.setStateIncomplete()
      })
    },
    _getOrderedSectionList: function() {
      return this.get("model").get("requiresShipping") ? this._customFormSection ? [this._billingSection, this._shippingSection, this._customFormSection, this._paymentSection] : [this._billingSection, this._shippingSection, this._paymentSection] : this._customFormSection ? [this._billingSection, this._customFormSection, this._paymentSection] : [this._billingSection,
        this._paymentSection]
    },
    _submit: function() {
      if (this._validateSubmit()) if (this.lock(), this._serializeCustomFormSubmission(), 0 === this.get("model").get("grandTotalCents")) this.get("contentBox").one("form").submit();
      else {
        var a = this._billingSection.getValues(),
          d = this._paymentSection.getValues();
        Stripe.createToken({
          name: d.cardHolderName,
          number: d.cardNumber,
          cvc: d.cvc,
          exp_month: d.cardExpiryMonth,
          exp_year: d.cardExpiryYear,
          address_line1: a.billingAddress1,
          address_line2: a.billingAddress2,
          address_state: a.billingState,
          address_city: a.billingCity,
          address_country: a.billingCountry,
          address_zip: a.billingZip
        }, b.bind(this._stripeResponseHandler, this))
      }
    },
    _validateSubmit: function() {
      var a = this._paymentSection.validate();
      if (0 < a.length) return this._paymentSection.renderErrors(a), !1;
      if ((a = this._customFormSection) && !a.validate()) return !1;
      var a = this.get("model"),
        d = a.get("grandTotalCents");
      return 0 < d && 90 > d ? (new b.Squarespace.Widgets.Alert({
        "strings.title": "Cannot Complete Order",
        "strings.message": "Your order grand total must be at least " + b.Squarespace.Commerce.currencySymbol() + "0.90 to continue."
      }), !1) : Static.SQUARESPACE_CONTEXT.websiteSettings.storeSettings.storeState === b.Squarespace.StoreStates.NOT_CONNECTED ? (new b.Squarespace.Widgets.Alert({
        "strings.title": "Payments Not Connected",
        "strings.message": "This store has not connected a payment gateway. Checkout is disabled and you cannot complete this purchase."
      }), !1) : 0 === a.get("totalQuantity") ? (new b.Squarespace.Widgets.Alert({
        "strings.title": "Cart Empty",
        "strings.message": "Your cart is empty. You cannot complete this purchase."
      }), !1) : !0
    },
    _serializeCustomFormSubmission: function() {
      this._customFormSection && this.get("contentBox").one("form").one('input[name="customFormSubmission"]').set("value", JSON.stringify(this._customFormSection.getCustomFormSubmission()))
    },
    _updateCartShippingLocationFromShipping: function() {
      this._shippingUpdateQueue.add({
        fn: this._seriallyUpdateShippingLocation,
        context: this,
        args: [this._shippingSection.getLocationForShipping()]
      }).run()
    },
    _updateCartShippingLocationFromBilling: function() {
      this._shippingUpdateQueue.add({
        fn: this._seriallyUpdateShippingLocation,
        context: this,
        args: [this._billingSection.getLocationForShipping()]
      }).run()
    },
    _seriallyUpdateShippingLocation: function(a) {
      this._shippingUpdateQueue.pause();
      this.get("model").updateShippingLocation(a, b.bind(function() {
        this._shippingUpdateQueue.run()
      }, this))
    },
    _stripeResponseHandler: function(a, b) {
      var c = b.error,
        f = this.get("contentBox").one("form");
      c ? (this.unlock(), f = this._paymentSection, f.renderErrors([{
        type: f.getProperty("FIELD_ERROR_TYPES").STRIPE,
        message: c.message
      }])) : (this._paymentSection.setValues({
        stripeToken: b.id
      }),
      f.submit())
    }
  }, {
    CSS_PREFIX: "sqs-checkout-form",
    ATTRS: {
      model: {
        value: null,
        validator: function(a) {
          return b.instanceOf(a, b.Squarespace.Models.ShoppingCart)
        }
      },
      storeCurrencyCode: {
        value: null
      },
      countriesAllowed: {
        value: []
      },
      optionalFields: {
        value: null
      },
      inTestMode: {
        validator: b.Squarespace.AttrValidators.isBoolean
      },
      enableMailingListOptInByDefault: {
        validator: b.Squarespace.AttrValidators.isBoolean
      }
    }
  })
}, "1.0", {
  requires: "base node squarespace-attr-validators squarespace-checkout-form-billing squarespace-checkout-form-payment squarespace-checkout-form-shipping squarespace-commerce-utils squarespace-models-shopping-cart squarespace-ss-widget squarespace-ui-base squarespace-util squarespace-widgets-alert squarespace-checkout-form-payment squarespace-checkout-form-custom-form".split(" ")
});
YUI.add("datatable-base", function(b, c) {
  b.DataTable.Base = b.Base.create("datatable", b.Widget, [b.DataTable.Core], {
    delegate: function() {
      var a = this.get("contentBox");
      return a.delegate.apply(a, arguments)
    },
    destructor: function() {
      this.view && this.view.destroy()
    },
    getCell: function() {
      return this.view && this.view.getCell && this.view.getCell.apply(this.view, arguments)
    },
    getRow: function() {
      return this.view && this.view.getRow && this.view.getRow.apply(this.view, arguments)
    },
    _afterDisplayColumnsChange: function(a) {
      this._extractDisplayColumns(a.newVal || [])
    },
    bindUI: function() {
      this._eventHandles.relayCoreChanges = this.after(["columnsChange", "dataChange", "summaryChange", "captionChange", "widthChange"], b.bind("_relayCoreAttrChange", this))
    },
    _defRenderViewFn: function(a) {
      a.view.render()
    },
    _extractDisplayColumns: function(a) {
      function d(a) {
        var g, h, k;
        g = 0;
        for (h = a.length; g < h; ++g) k = a[g], b.Lang.isArray(k.children) ? d(k.children) : c.push(k)
      }
      var c = [];
      d(a);
      this._displayColumns = c
    },
    initializer: function() {
      this.publish("renderView", {
        defaultFn: b.bind("_defRenderViewFn", this)
      });
      this._extractDisplayColumns(this.get("columns") || []);
      this.after("columnsChange", b.bind("_afterDisplayColumnsChange", this))
    },
    _relayCoreAttrChange: function(a) {
      this.view.set("data" === a.attrName ? "modelList" : a.attrName, a.newVal)
    },
    renderUI: function() {
      var a = this,
        d = this.get("view");
      d && (this.view = new d(b.merge(this.getAttrs(), {
        host: this,
        container: this.get("contentBox"),
        modelList: this.data
      }, this.get("viewConfig"))), this._eventHandles.legacyFeatureProps || (this._eventHandles.legacyFeatureProps = this.view.after({
        renderHeader: function(b) {
          a.head = b.view;
          a._theadNode = b.view.theadNode;
          a._tableNode = b.view.get("container")
        },
        renderFooter: function(b) {
          a.foot = b.view;
          a._tfootNode = b.view.tfootNode;
          a._tableNode = b.view.get("container")
        },
        renderBody: function(b) {
          a.body = b.view;
          a._tbodyNode = b.view.tbodyNode;
          a._tableNode = b.view.get("container")
        },
        renderTable: function() {
          var b = this.get("container");
          a._tableNode = this.tableNode || b.one("." + this.getClassName("table") + ", table");
          a._captionNode = this.captionNode || b.one("caption");
          a._theadNode || (a._theadNode = b.one("." + this.getClassName("columns") + ", thead"));
          a._tbodyNode || (a._tbodyNode = b.one("." + this.getClassName("data") + ", tbody"));
          a._tfootNode || (a._tfootNode = b.one("." + this.getClassName("footer") + ", tfoot"))
        }
      })), this.view.addTarget(this))
    },
    syncUI: function() {
      this.view && this.fire("renderView", {
        view: this.view
      })
    },
    _validateView: function(a) {
      return null === a || b.Lang.isFunction(a) && a.prototype.render
    }
  }, {
    ATTRS: {
      view: {
        value: b.DataTable.TableView,
        validator: "_validateView"
      },
      viewConfig: {}
    }
  });
  b.DataTable = b.mix(b.Base.create("datatable",
  b.DataTable.Base, []), b.DataTable)
}, "3.17.2", {
  requires: "datatable-core datatable-table datatable-head datatable-body base-build widget".split(" "),
  skinnable: !0
});
YUI.add("squarespace-checkout-shopping-cart", function(b) {
  b.namespace("Squarespace.Widgets");
  var c = b.Squarespace.Widgets.CheckoutShoppingCart = b.Base.create("checkoutShoppingCart", b.Squarespace.Widgets.TableShoppingCart, [], {
    renderUI: function() {
      c.superclass.renderUI.call(this);
      this._spinner = new b.Squarespace.Spinner({
        render: this.get("contentBox").one(".loading-spinner"),
        size: 50,
        color: "dark"
      })
    },
    bindUI: function() {
      c.superclass.bindUI.call(this);
      var a = this.get("model");
      a.on("recalculate-start", this._setLoadingState,
      this);
      a.on("recalculate-end", this._setLoadedState, this)
    },
    syncUI: function() {
      c.superclass.syncUI.call(this);
      var a = this.get("model"),
        d = this.get("contentBox");
      1 === a.get("entries").length && d.addClass("single-item");
      d.one(".tax .price").setContent(b.Squarespace.Commerce.moneyString(a.get("taxCents")));
      d.one(".shipping .price").setContent(b.Squarespace.Commerce.moneyString(a.get("shippingCostCents")));
      var e = d.one(".shipping .label"),
        f = a.get("shippingLocation").zip;
      b.Lang.isValue(f) && "" !== f ? e.setContent("Shipping (" + a.get("shippingLocation").zip + ")") : e.setContent("Shipping");
      e = a.get("discountCents");
      f = d.one(".discounts");
      f.one(".price").setContent("- " + b.Squarespace.Commerce.moneyString(e));
      0 === e ? f.hide() : f.show();
      d.one(".grand-total .price").setContent(b.Squarespace.Commerce.moneyString(a.get("grandTotalCents")))
    },
    lock: function() {
      var a = this.get("contentBox");
      a.addClass("locked");
      a.all("input").set("disabled", !0)
    },
    unlock: function() {
      var a = this.get("contentBox");
      a.removeClass("locked");
      a.all("input").set("disabled", !1)
    },
    _setLoadingState: function() {
      var a = this.get("contentBox");
      a.all("input").setAttribute("disabled", !0);
      a.addClass("loading-cart")
    },
    _setLoadedState: function() {
      var a = this.get("contentBox");
      b.later(350, this, function() {
        a.all("input").removeAttribute("disabled");
        a.removeClass("loading-cart")
      })
    }
  }, {
    CSS_PREFIX: "sqs-checkout-shopping-cart",
    HANDLEBARS_TEMPLATE: "checkout-shopping-cart.html",
    HANDLEBARS_ITEM_TEMPLATE: "checkout-shopping-cart-item.html",
    ATTRS: {
      continueShoppingUrl: {
        valueFn: function() {
          return Static.SQUARESPACE_CONTEXT.website.authenticUrl
        }
      }
    }
  })
}, "1.0", {
  requires: "base node squarespace-commerce-utils squarespace-table-shopping-cart squarespace-ui-templates squarespace-hb-money-string squarespace-checkout-form-shipping-options squarespace-checkout-shopping-cart-template squarespace-checkout-shopping-cart-item-template squarespace-checkout-coupon-list squarespace-spinner".split(" ")
});
YUI.add("squarespace-widgets-data-widget", function(b) {
  b.namespace("Squarespace.Widgets");
  var c = b.Squarespace.Widgets.DataWidget = b.Base.create("dataWidget", b.Squarespace.Widgets.SSWidget, [], {
    initializer: function(a) {
      a.dataState || (this.getProperty("ASYNC_DATA") ? this.set("dataState", this.getProperty("DATA_STATES").INITIALIZED) : this.set("dataState", this.getProperty("DATA_STATES").LOADED))
    },
    renderUI: function() {
      c.superclass.renderUI.call(this);
      this._updateDataStateClassName()
    },
    bindUI: function() {
      c.superclass.bindUI.call(this);
      var a = this.get("id");
      this.after(a + "|dataChange", function(a) {
        a.noSyncUI || this.syncUI()
      }, this);
      this.after(a + "|dataStateChange", this._updateDataStateClassName, this)
    },
    _updateDataStateClassName: function() {
      var a = this.get("boundingBox"),
        d = this.get("dataState");
      b.Object.each(this.getProperty("DATA_STATES"), function(b) {
        a.removeClass("data-state-" + b)
      });
      a.addClass("data-state-" + d)
    },
    setLoadingState: function() {
      return this.set("dataState", this.getProperty("DATA_STATES").LOADING)
    },
    setLoadedState: function() {
      return this.set("dataState",
      this.getProperty("DATA_STATES").LOADED)
    },
    setLoadFailedState: function() {
      return this.set("dataState", this.getProperty("DATA_STATES").LOAD_FAILED)
    },
    loadedSuccessfully: function() {
      return this.get("dataState") === this.getProperty("DATA_STATES").LOADED
    },
    isLoading: function() {
      return this.get("dataState") === this.getProperty("DATA_STATES").LOADING
    },
    loadFailed: function() {
      return this.get("dataState") === this.getProperty("DATA_STATES").LOAD_FAILED
    }
  }, {
    CSS_PREFIX: "sqs-data-widget",
    ASYNC_DATA: !1,
    DATA_STATES: {
      INITIALIZED: "initialized",
      LOADING: "loading",
      LOADED: "loaded",
      LOAD_FAILED: "load-failed"
    },
    ATTRS: {
      data: {
        value: null,
        validator: function(a) {
          return b.Lang.isUndefined(a) ? (console.warn(this.name + ": Will not set data to undefined."), !1) : !0
        }
      },
      dataState: {
        valueFn: function() {
          return this.getProperty("DATA_STATES").INITIALIZED
        }
      },
      preventRenderTemplate: {
        value: !1,
        validator: b.Squarespace.AttrValidators.isBoolean
      }
    }
  })
}, "1.0", {
  requires: ["base", "node", "widget", "squarespace-ss-widget", "squarespace-attr-validators"]
});
YUI.add("squarespace-dialog-fields-generators", function(b) {
  function c(a) {
    return !{
      13: !0,
      27: !0,
      16: !0,
      17: !0,
      18: !0,
      20: !0,
      36: !0,
      35: !0,
      33: !0,
      34: !0,
      37: !0,
      38: !0,
      39: !0,
      40: !0,
      112: !0,
      113: !0,
      114: !0,
      115: !0,
      116: !0,
      117: !0,
      118: !0,
      119: !0,
      120: !0,
      121: !0,
      122: !0,
      123: !0,
      45: !0,
      91: !0,
      144: !0
    }[a]
  }
  b.namespace("Squarespace.DialogFieldGenerators");
  b.Squarespace.DialogFieldGenerators["multi-frame"] = Class.extend(b.Squarespace.DialogField, {
    initialize: function(a, d, c) {
      this._super(a, d, c);
      this.frames = {};
      this.activeEl = null;
      this.value = this.config.initialFrame;
      this.panel.bodyEvents.push(b.on("click", this.onClick, this.html, this))
    },
    onClick: function(a) {
      this.panel.fire("datachange", this)
    },
    show: function(a) {
      var d = this.frames[a];
      if (this.activeEl) {
        if (this.activeEl == d) return;
        this.activeEl.a && this.activeEl.a.stop();
        var c = this.activeEl.fields;
        this.activeEl.a = this._anim({
          node: this.activeEl.el,
          to: {
            height: 0
          },
          duration: 0.3,
          easing: b.Easing.easeOutStrong
        });
        c.forEach(this.setInActiveFrame(!1), this);
        this.activeEl.a.on("end", function(a) {
          a.target.get("node").addClass("hidden")
        },
        this);
        this.activeEl.a.run()
      }
      d.a && d.a.stop();
      this.activeEl && d.el.ancestor().insertBefore(d.el, this.activeEl.el.next());
      d.el.removeClass("hidden");
      c = d.el.getStyle("height");
      d.el.setStyles({
        height: null
      });
      d.realHeight = d.el.get("offsetHeight");
      d.el.setStyles({
        height: c
      });
      d.a = this._anim({
        node: d.el,
        to: {
          height: d.realHeight
        },
        duration: 0.3,
        easing: b.Easing.easeOutStrong
      });
      d.a.on("end", function(a, b) {
        for (var d = 0; d < b.fields.length; ++d) {
          var c = this.panel.fields[b.fields[d].name];
          c && c.updateInlineTitle && c.updateInlineTitle()
        }
        this.focusCurrentFrame()
      },
      this, d);
      d.a.run();
      d.fields.forEach(this.setInActiveFrame(!0), this);
      this.activeEl = d;
      this.value = a
    },
    focusCurrentFrame: function() {
      for (var a, d = 0; d < this.activeEl.fields.length; ++d) if (a = this.activeEl.fields[d], a = this.panel.fields[a.name || b.Object.getValue(a, ["config", "name"])], b.Lang.isValue(a) && a.getNode) {
        a.getNode().focus();
        break
      }
    },
    setInActiveFrame: function(a) {
      return function(b) {
        b.name && (b = this.panel.getField(b.name), b.inActiveFrame = a, b.fire("mutli-frame-focus"))
      }
    },
    append: function(a, d, c, f) {
      for (var g in this.config.frames) {
        d = this.config.frames[g];
        var h = b.DB.DIV("frame-wrapper clear");
        c.append(h);
        this.panel._renderFields(a, d.fields, h, f);
        this.frames[g] = this.config.frames[g];
        this.frames[g].el = h;
        this.frames[g].realHeight = h.get("offsetHeight");
        g != this.config.initialFrame ? (h.setStyle("height", "0px"), h.addClass("hidden"), this.frames[g].visible = !1, this.config.frames[g].fields.forEach(this.setInActiveFrame(!1), this)) : (this.activeEl = this.frames[g], this.frames[g].visible = !0, this.config.frames[g].fields.forEach(this.setInActiveFrame(!0),
        this))
      }
    }
  });
  b.Squarespace.DialogFieldGenerators.select = Class.extend(b.Squarespace.DialogField, {
    initialize: function(a, d, c) {
      this._super(a, d, c);
      this.config.options || (this.config.options = [], console.warn("Select '" + this.config.name + "' has no options."));
      this.config.icon || (this.config.icon = "", this.noIcon = !0);
      this.control = b.DB.DIV("field-input-wrapper select " + (this.config.className ? this.config.className : ""), {
        html: "&nbsp;"
      });
      this.config.width ? b.Lang.isNumber(this.config.width) ? 1 <= this.config.width && (this.selectElWidth = this.config.width + "px") : b.Lang.isString(this.config.width) ? this.selectElWidth = this.config.width : console.log("dialog: [DialogField] Width is not a number or string", this) : this.selectElWidth = !1;
      this.underlyingControl = b.DB.SELECT("input", {
        tabIndex: c.getNextTabIndex(),
        style: "position: absolute; top: 0; left: 0px; opacity: 0"
      });
      var f, g;
      for (g in this.config.options) d[this.config.name] == g && (this.value = g), f || (f = g), this.underlyingControl.append(b.DB.OPTION({
        html: this.config.options[g].title,
        value: g
      }));
      if (b.Lang.isUndefined(this.value) || b.Lang.isNull(this.value) || !this.config.options[this.value]) null != this.config.defaultValue && !b.Lang.isUndefined(this.config.defaultValue) && this.config.options[this.config.defaultValue] ? this.value = this.config.defaultValue : this.value = f;
      this.html = b.DB.DIV("field-wrapper select-field-wrapper clear thin dialog-field-" + this.config.name, this.config.title ? b.DB.DIV("field-title", {
        html: this.config.title
      }) : null, b.DB.DIV("field-wrapper-inner clear" + (this.config.icon ? " icon " + this.config.icon : ""), {
        style: "position: relative;" + (this.selectElWidth ? "width: " + this.selectElWidth : "")
      }, b.DB.DIV("field-rhs", this.control), this.underlyingControl), this.config.description ? b.DB.DIV("field-description-wrapper", b.DB.DIV("field-description", {
        html: this.config.description
      })) : null);
      this.setValue(this.value);
      this._subscribe([[this.html.one(".field-wrapper-inner"), "click", this.onClick], [this.underlyingControl, "change", this.onChange]]);
      this.config.onChange && this._subscribe([[this.underlyingControl, "change", this.config.onChange]]);
      b.Object.isEmpty(this.config.options) && this.html.setStyle("display", "none");
      a.hidden && this.temporaryHide(!0)
    },
    didDataChange: function() {
      return b.Lang.isNumber(Number(this.initialData[this.getName()])) && b.Lang.isNumber(this.getValue()) ? Number(this.initialData[this.getName()]) != Number(this.getValue()) : this.initialData[this.getName()] != this.getValue()
    },
    updateOptions: function(a, d) {
      a = a || {};
      this.html.setStyle("display", b.Object.isEmpty(a) ? "none" : null);
      this.config.options = a;
      this.underlyingControl.set("innerHTML", "");
      b.Object.each(a, function(d,
      c) {
        this.underlyingControl.append(b.DB.OPTION({
          html: a[c].title,
          value: c
        }))
      }, this);
      !b.Lang.isUndefined(d) && null != a[d] && (this.setValue(d), this.html.one(".field-input-wrapper.select").setContent(this.getValueTitle(d)))
    },
    onClick: function(a) {
      a.halt();
      if (!this._disabled && "SELECT" != a.target.get("tagName")) {
        if (a.target.hasClass("field-wrapper-inner")) a = a.target;
        else {
          if (a.target.ancestors(".field-wrapper-inner").isEmpty()) return;
          a = a.target.ancestors(".field-wrapper-inner").item(0)
        }
        a.one("select").simulate("mousedown")
      }
    },
    show: function() {
      if (!this.noIcon) {
        var a = parseInt(this.html.get("offsetWidth"), 10) - parseInt(this.html.getStyle("paddingLeft"), 10) - 5;
        this.underlyingControl.setStyle("width", a + "px")
      }
    },
    onChange: function() {
      this.setValue(this.underlyingControl.get("value"));
      this.fire("change", this.getValue());
      this.dialog.fire("datachange", this)
    },
    hideDescription: function() {
      this.html.one(".field-description").hide()
    },
    showDescription: function(a) {
      this.html.one(".field-description").show();
      this.html.one(".field-description").setContent(a)
    },
    getDescription: function() {
      return this.html.one(".field-description")
    },
    rollback: function() {
      this.setValue(this.previousValue);
      this.html.one(".field-input-wrapper.select").setContent(this.getValueTitle(this.getValue()))
    },
    setValue: function(a) {
      this.previousValue = this.value;
      this.value = a;
      this.underlyingControl.set("value", a);
      b.Object.isEmpty(this.config.options) || (this.control.setContent(this.getValueTitle()), this.getValueIsEmpty() ? this.html.addClass("empty-value") : this.html.removeClass("empty-value"))
    },
    getPreviousValue: function() {
      return this.previousValue
    },
    getValue: function() {
      return this.underlyingControl.get("value")
    },
    getValueTitle: function() {
      return this.config.options[this.underlyingControl.get("value")].title
    },
    getValueIsEmpty: function() {
      return this.config.options[this.underlyingControl.get("value")].empty
    },
    disable: function() {
      this._disabled = !0;
      this.underlyingControl.setAttribute("disabled", "disabled");
      this.html.addClass("disabled")
    },
    enable: function() {
      this._disabled = !1;
      this.underlyingControl.removeAttribute("disabled");
      this.html.removeClass("disabled")
    }
  });
  b.Squarespace.DialogFieldGenerators["multi-option"] = Class.extend(b.Squarespace.DialogField, {
    initialize: function(a, d, c) {
      this._super(a, d, c);
      this.config.options = this.config.options.filter(function(a) {
        return !b.Lang.isUndefined(a) && !b.Lang.isNull(a)
      });
      this.value = d[this.config.name] ? d[this.config.name] : this.config.defaultValue;
      this.config.columns || (this.config.columns = this.config.options.length);
      !this.value && this.config.multi && (this.value = []);
      this.options = [];
      this.tips = [];
      this.remover = {};
      this.dds = [];
      for (a = 0; a < this.config.options.length; ++a) if (d = this.config.options[a]) {
        if (this.config.multi) {
          c = !1;
          for (var f = 0; f < this.value.length; ++f) if (this.value[f] == d.value) {
            c = !0;
            break
          }
        } else c = d.value == this.value;
        if (!d.disabled || !this.config.hideDisabled && d.disabled) c = b.DB.DIV("multioption-element option-" + d.value + (c ? " active" : "") + (this.config.buttonStyle ? " button" : "") + (this.config.optionStyle ? " " + this.config.optionStyle : "") + (d.optionStyle ? " " + d.optionStyle : "") + (d.disabled ? " disabled" : ""), {
          html: '<div class="text clear"><div class="title">' + d.title + "</div>" + (d.description ? '<div class="description">' + d.description + "</div>" : "") + "</div>",
          style: "background-position: center " + this.config.iconDropPx + "px; padding: " + this.config.optionPadding,
          data: {
            value: d.value
          }
        }), d.icon && c.setStyle("backgroundImage", "url(" + d.icon + ")"), this.config.height && c.setStyle("height", this.config.height), !d.title && !d.description && c.setStyle("visibility", "hidden"), this.options.push(c), this.remover[d.value] = c, d.value == this.value && (this.activeEl = c), this.config.dragTray && (c.setStyle("cursor", "move"), c.setData(d), c = (new b.DD.Drag({
          node: c
        })).plug(b.Plugin.DDProxy, {
          moveOnEnd: !1,
          hideOnEnd: !1,
          borderStyle: "none"
        }), this.dds.push(c), c.on("drag:start", function(a, d, c) {
          a = a.target.get("node");
          b.one("body").append(a);
          a.addClass("free-block reattached-node");
          this.dds.splice(this.dds.indexOf(d), 1);
          this.panel.close()
        }, this, c, d.value), c.on("drag:end", function(a, d, c) {
          a = b.DD.DDM.activeDrag;
          c = a.get("node");
          a.get("dragNode").transition({
            easing: "ease-out",
            transform: {
              value: !c.placeholder ? "scale(1.04)" : "scale(.6)",
              duration: 0.4
            },
            opacity: {
              value: 0,
              duration: 0.4
            }
          }, function() {
            this.setStyle("display", "none");
            this.setStyle("transform", "");
            this.setStyle("opacity", "")
          });
          d.destroy();
          (d = b.one("body").one(".reattached-node")) && d.remove()
        }, this, c, d.value), this.config.attachDragHandlers(c))
      }
      this.empty = b.DB.DIV("multioption-placeholder", {
        style: this.config.backgroundIcon ? "backgroundImage: url(" + this.config.backgroundIcon + ")" : ""
      });
      this.elements = b.DB.DIV("field-input-wrapper",
      0 < this.options.length ? this.options : this.empty);
      this.control = this.html = b.DB.DIV("field-wrapper multioption-field-wrapper clear " + (this.config.alwaysShowTitles ? " show-titles " : "") + (this.config.style ? this.config.style : ""), this.config.padding ? {
        style: "padding-top: " + this.config.padding[0] + "; padding-bottom: " + this.config.padding[1] + ";"
      } : null, this.config.title && ("left" == this.config.titleStyle || "top" == this.config.titleStyle) ? b.DB.DIV("field-lhs " + this.config.titleStyle, {
        html: this.config.title
      }) : null, b.DB.DIV("field-rhs",
      this.elements, this.config.description ? b.DB.DIV("field-description-wrapper", b.DB.DIV("field-description " + this.config.titleStyle, {
        html: this.config.description
      })) : null));
      this.panel.bodyEvents.push(b.on("click", this.onClick, this.html, this));
      this.panel.bodyEvents.push(b.on("contextmenu", this.onRightClick, this.html, this))
    },
    show: function() {
      this.config.options.forEach(function(a) {
        var d = b.one(".option-" + a.value);
        a.tip && d && this.tips.push(new b.Squarespace.ToolTip({
          target: d,
          alwaysEnable: !0,
          title: a.tip.title,
          body: a.tip.text,
          dialogTooltip: !0
        }))
      }, this)
    },
    onRightClick: function(a) {
      var b = a.target.ancestor(".multioption-element", !0);
      if (b) {
        var c = this.value;
        this.value = b.data().value;
        this.panel.fire("contextmenu", this);
        this.value = c;
        a.halt()
      }
    },
    onClick: function(a) {
      this.previousValue = this.value;
      var b = a.target.ancestor(".multioption-element", !0);
      a.halt();
      if (b) {
        a = b.getData();
        var c = a.value;
        c && (b.hasClass("disabled") ? this.fire("disabled-click", a.value) : (this.panel.fire("multioption-click", b.data().value), this.config.multi ? (b.toggleClass("active"), this.value = [], this.html.all(".multioption-element.active").each(function(a) {
          this.value.push(a.data().value)
        }, this)) : (this.activeEl && this.activeEl.removeClass("active"), b.addClass("active"), this.activeEl = b, this.value = c), this.config.linkFrame && this.panel.getField(this.config.linkFrame).show(this.value), this.panel.fire("datachange", this, a)))
      }
    },
    resize: function() {
      var a = parseInt(this.html.getStyle("width"), 10);
      "left" == this.config.titleStyle && (a -= 110);
      for (var a = (a - 2 * this.config.columns) / this.config.columns, b = 0; b < this.options.length; ++b) this.options[b].setStyle("width", a + "px")
    },
    add: function(a, d, c) {
      this.empty.setStyle("display", "none");
      d = b.DB.DIV("multioption-element " + (a == this.value ? "active" : "") + (this.config.buttonStyle ? "button" : ""), {
        html: '<div class="multioption-thumbnail" style="background-image:url(' + d + ')"></div><div class="multioption-title">' + c + "</div>",
        style: "padding: " + this.config.optionPadding + (this.config.height ? "height: " + this.config.height + "px;" : ""),
        data: {
          value: a
        }
      });
      this.remover[a] = d;
      this.elements.appendChild(this.remover[a])
    },
    remove: function(a) {
      this.remover[a] && (this.remover[a].remove(), delete this.remover[a])
    },
    clear: function() {
      for (var a in this.remover)!1 !== a && void 0 !== a && (this.remover[a].remove(), delete this.remover[a]);
      this.empty.setStyle("display", "block")
    },
    setHighlight: function(a) {
      this.remover[a] && this.remover[a].addClass("highlighted")
    },
    removeHighlight: function(a) {
      this.remover[a] && this.remover[a].removeClass("highlighted")
    },
    clearHighlight: function(a) {
      this.html.all(".multioption-element").removeClass("highlighted")
    },
    _destroy: function() {
      var a;
      this._super();
      for (a = 0; a < this.tips.length; ++a) this.tips[a].destroy();
      for (a = 0; a < this.dds.length; ++a) this.dds[a].destroy()
    }
  });
  b.Squarespace.DialogFieldGenerators.check = Class.extend(b.Squarespace.DialogField, {
    initialize: function(a, d, c) {
      this._super(a, d, c);
      this.active = (void 0 === d[this.config.name] ? this.config.defaultValue : d[this.config.name]) ? !0 : !1;
      this.html = b.DB.DIV("field-wrapper check-field-wrapper clear thin", this.config.padding ? {
        style: "padding-top: " + this.config.padding[0] + "; padding-bottom: " + this.config.padding[1] + ";"
      } : null, b.DB.DIV("check-element " + (this.active ? "active" : ""), this.config.title ? b.DB.DIV("field-title", {
        html: this.config.title
      }) : null, this.config.description ? b.DB.DIV("field-description-wrapper", b.DB.DIV("field-description", {
        html: this.config.description
      })) : null));
      a.hidden && this.temporaryHide(!0);
      this.panel.bodyEvents.push(b.on("click", this.onClick, this.html, this))
    },
    onClick: function(a) {
      this.setValue(!this.active);
      this.panel.fire("datachange", this);
      this.fire("changed",
      this.getValue())
    },
    setValue: function(a) {
      (this.active = a) ? this.html.one(".check-element").addClass("active") : this.html.one(".check-element").removeClass("active")
    },
    getValue: function() {
      return this.active
    },
    didDataChange: function() {
      return !this.initialData[this.getName()] && !this.getValue() ? !1 : this._super()
    }
  });
  b.Squarespace.DialogFieldGenerators.text = Class.extend(b.Squarespace.DialogField, {
    initialize: function(a, d, c) {
      this.hideAnim = null;
      this._super(a, d, c);
      d = d[this.config.name];
      if (b.Lang.isUndefined(d) || b.Lang.isNull(d)) d = "";
      this.control = b.DB.INPUT("field-input " + (this.config.className ? this.config.className : ""), {
        type: this.config.password ? "password" : "text",
        spellcheck: "false",
        tabIndex: this.panel.getNextTabIndex(),
        value: d,
        style: (a.bold ? "font-weight: bold;" : "") + (a.width && 1 < a.width ? "width: " + a.width + "px;" : ""),
        maxLength: a.maxLength ? a.maxLength : "",
        placeholder: this.config.placeholder ? this.config.placeholder : ""
      });
      this.config.readonly && this.control.setAttribute("readonly", !0);
      this.config.titleStyle || (this.config.titleStyle = "top");
      this.config.style || (this.config.style = "normal");
      this.config.filter || (this.config.filter = "none");
      this.config.icon || (this.config.icon = "none");
      if ("minor" == this.config.style) this.html = b.DB.DIV("field-wrapper text-field-wrapper clear minor dialog-field-" + this.config.name, b.DB.DIV("field-rhs", b.DB.DIV("field-input-wrapper", this.control), this.config.title && "top" == this.config.titleStyle ? b.DB.DIV("field-lhs", {
        html: this.config.title
      }) : null, this.config.description ? b.DB.DIV("field-description-wrapper",
      b.DB.DIV("field-description", {
        html: this.config.description
      })) : null));
      else if ("thin" == this.config.style || "small" == this.config.style) this.prefixEl = b.DB.DIV("field-lhs", {
        html: this.config.prefix
      }), this.html = b.DB.DIV("field-wrapper text-field-wrapper clear " + this.config.style + " dialog-field-" + this.config.name, this.config.title && "small" != this.config.style ? b.DB.DIV("field-title", {
        html: this.config.title
      }) : null, b.DB.DIV("field-wrapper-inner clear " + this.config.icon, this.prefixEl, b.DB.DIV("field-rhs", b.DB.DIV("field-input-wrapper",
      this.control))), this.config.description ? b.DB.DIV("field-description-wrapper", b.DB.DIV("field-description", {
        html: this.config.description
      })) : null), this.errorFlyoutAnchor = this.html.one(".field-wrapper-inner");
      else if ("normal" == this.config.style || "major" == this.config.style || "title" == this.config.style) {
        if (this.config.prefixText) this.titleEl = b.DB.DIV("field-lhs left", {
          html: this.config.prefixText
        });
        else if (this.config.title && ("left" == this.config.titleStyle || "top" == this.config.titleStyle)) this.titleEl = b.DB.DIV("field-lhs " + this.config.titleStyle, {
          html: this.config.title
        });
        this.html = b.DB.DIV("field-wrapper text-field-wrapper clear " + this.config.style + " dialog-field-" + this.config.name, this.titleEl, b.DB.DIV("field-rhs", b.DB.DIV("field-input-wrapper", this.control), this.config.description ? b.DB.DIV("field-description-wrapper", b.DB.DIV("field-description " + this.config.titleStyle, {
          html: this.config.description
        })) : null))
      }
      this.config.hide && this.html.setStyle("display", "none");
      this.panel.bodyEvents.push(b.on("paste", this.onPaste,
      this.control, this));
      this.panel.bodyEvents.push(b.on("keyup", this.onKeyUp, this.control, this));
      this.panel.bodyEvents.push(b.on("keypress", this.applyKeyFilter, this.control, this));
      this.panel.bodyEvents.push(b.on("keydown", this.onKeyDown, this.control, this));
      this.panel.bodyEvents.push(b.on("key", this.onSubmit, this.control, "down:13", this));
      this._subscribe([[this.control, "change", this.onChange]]);
      a.mailcheck && this.control.plug(b.Squarespace.Plugin.MailCheck, {
        field: this
      })
    },
    onChange: function() {
      this.fire("change",
      this.getValue());
      this.dialog.fire("datachange", this)
    },
    onPaste: function(a) {
      b.later(1, this, function() {
        this.updateInlineTitle();
        this.updateTextSuffix()
      })
    },
    onKeyDown: function(a) {
      9 === a.keyCode ? (this.updateInlineTitle(), this.updateTextSuffix()) : c(a.keyCode) && this.hideInlineTitle();
      27 !== a.keyCode && a.stopPropagation()
    },
    applyKeyFilter: function(a) {
      if (this.config.keyFilter && (0 === b.UA.gecko || a._event.isChar)) {
        var d = String.fromCharCode(a.which);
        !d.match(RegExp(this.config.keyFilter)) && 0 < d.length && (this.config.keyFilterExplanation && this.showError(this.config.keyFilterExplanation), a.halt())
      }
    },
    onKeyUp: function(a) {
      if ("url-slug" === this.config.filter || "url-slug-with-slash" == this.config.filter) {
        var d, e = this.control.get("value");
        d = "url-slug" === this.config.filter ? b.Squarespace.Utils.createUrl(e) : b.Squarespace.Utils.createUrlWithSlash(e);
        var f = this.control._node.selectionStart - (32 == a.keyCode ? 0 : 1);
        e != d && (this.control.set("value", d), this.control._node.selectionStart = f, this.control._node.selectionEnd = f)
      }
      this.config.syncedField && this.panel.getField(this.config.syncedField).setValue(this.getValue());
      c(a.keyCode) && (this.panel.fire("keyup", this), this.updateInlineTitle(), this.updateTextSuffix(), this.panel.fire("datachange", this), this.clearError(), a.stopPropagation());
      13 == a.keyCode && this.fire("keyup-enter")
    },
    onSubmit: function(a) {
      a.halt(!0);
      this.config.submitOnEnter && this.dialog.saveAndClose();
      this.config.saveOnEnter && this.panel.save()
    },
    isEmpty: function() {
      return !this.control ? !1 : 0 === this.control.get("value").length
    },
    enable: function() {
      this.html.removeClass("disabled");
      this.control.set("disabled", "")
    },
    disable: function() {
      this.html.addClass("disabled");
      this.control.set("disabled", "true")
    },
    showInlineTitle: function() {
      null !== this.hideAnim && (this.hideAnim.stop(), this.hideAnim = null);
      if (null == this.control.getDOMNode() || null == this.html.getDOMNode()) console.warn("Text field DOM nodes in invalid states. Bailing.");
      else {
        this.inlineTitle || (this.inlineTitle = b.DB.DIV("inline-field-title " + this.config.style, {
          html: this.config.title
        }), this.inlineTitle.event = b.on("click", this.focus, this.inlineTitle, this), null !== this.html.getDOMNode() ? b.one(this.html).append(this.inlineTitle) : console.warn('Text Field "html" property is null.'));
        var a, d;
        switch (this.config.style) {
        case "major":
          a = 16;
          break;
        case "title":
          a = 17;
          break;
        case "small":
          a = 3;
          break;
        default:
          a = 15
        }
        this.config.prefixText && (a += 110);
        d = this.control.get("offsetTop") + (this.control.get("offsetHeight") - this.inlineTitle.get("offsetHeight")) / 2;
        this.currentXY = [a + 26, d];
        this.inlineTitle.setStyles({
          left: this.currentXY[0] + "px",
          top: this.currentXY[1] + "px",
          opacity: 1
        })
      }
    },
    hideInlineTitle: function() {
      null !== this.hideAnim && (this.hideAnim.stop(), this.hideAnim = null);
      this.inlineTitle && this.inlineTitle.inDoc() && (this.hideAnim = this._anim({
        node: this.inlineTitle,
        from: {
          opacity: 1
        },
        to: {
          opacity: 0
        },
        duration: 0.05,
        easing: b.Easing.easeOutStrong
      }), this.hideAnim.on("end", function() {
        this.inlineTitle.event.detach();
        this.inlineTitle.remove();
        this.hideAnim = this.inlineTitle = null
      }, this), this.hideAnim.run())
    },
    updateInlineTitle: function() {
      "inline" == this.config.titleStyle && (this.isEmpty() ? this.showInlineTitle() : this.hideInlineTitle())
    },
    showTextSuffix: function() {
      var a = null,
        d = 10,
        c = 6;
      "major" == this.config.style && (d = 12, c = 11);
      "title" == this.config.style && (d = 19, c = 10);
      "small" == this.config.style && (c = d = 3);
      this.textSuffixwrapper || (a = b.DB.DIV("suffix inline-field-title " + this.config.style, {
        style: "position: relative;",
        html: this.config.suffix
      }), a.setStyles({
        top: c + "px",
        opacity: 1
      }), this.textSuffixwrapper = b.DB.DIV("suffix-wrapper", a), this.textSuffixwrapper.setStyle("height", this.control.get("offsetHeight") + "px"), this.textSuffixwrapper.setStyle("width",
      this.control.get("offsetWidth") + "px"), this.textSuffixwrapper.setStyle("top", c + "px"), this.textSuffixwrapper.setStyle("overflow", "hidden"), this.textSuffixwrapper.setStyle("position", "absolute"), this.textSuffixwrapper.event = this.textSuffixwrapper.on("click", this.focus, this), b.one(this.html.one(".field-input-wrapper")).append(this.textSuffixwrapper), this.measurementEl = b.DB.DIV("measurement-el inline-field-title " + this.config.style, {
        style: "position: absolute; opacity: 0;",
        html: ""
      }), b.one(this.html.one(".field-input-wrapper")).append(this.measurementEl));
      if (null === a) this.textSuffixwrapper.one(".suffix");
      a = this.getValue().replace(/ /g, ".");
      this.measurementEl.setContent(a);
      d += this.measurementEl.get("offsetWidth");
      a = this.control.get("offsetWidth");
      this.textSuffixwrapper.setStyle("width", a - d + "px");
      this.textSuffixwrapper.setStyle("left", 23 + d + "px");
      this.textSuffixwrapper.setStyle("top", "0px")
    },
    hideTextSuffix: function() {
      this.textSuffixwrapper && (this.textSuffixwrapper.remove(), this.textSuffixwrapper.event.detach(), this.textSuffixwrapper = null, this.measurementEl.remove())
    },
    updateTextSuffix: function() {
      this.config.suffix && (this.isEmpty() ? this.hideTextSuffix() : this.showTextSuffix())
    },
    resize: function() {
      var a;
      "left" == this.config.titleStyle ? (a = parseInt(this.html.getStyle("width"), 10) - parseInt(this.titleEl.get("offsetWidth"), 10), this.control.setStyle("width", a + "px")) : "thin" == this.config.style && (a = parseInt(this.html.getStyle("width"), 10) - parseInt(this.html.getStyle("paddingLeft"), 10) - parseInt(this.html.getStyle("paddingRight"), 10), this.control.setStyle("width", a - this.prefixEl.get("offsetWidth") - ("" !== this.config.icon ? 18 : 0) + "px"))
    },
    focus: function(a) {
      a && a.halt();
      this.control && this.control.focus()
    },
    show: function() {
      "inline" == this.config.titleStyle && (this.isEmpty() ? this.showInlineTitle() : this.updateTextSuffix());
      this._super()
    },
    hide: function() {
      "inline" == this.config.titleStyle && this.hideInlineTitle();
      this._super()
    },
    getErrors: function() {
      var a = this._super();
      return this.config.required && "" === this.control.get("value") ? ["Required field."] : a
    },
    getValue: function() {
      return this.control.get("value")
    },
    setValue: function(a) {
      this.control && (this.control.set("value", void 0 !== a ? a : ""), this.updateInlineTitle(), this.updateTextSuffix(), this.clearError())
    },
    _destroy: function() {
      this._super();
      this.testSuffixwrapper && this.testSuffixwrapper.event && this.testSuffixwrapper.event.detach()
    }
  });
  b.Squarespace.DialogFieldGenerators.textarea = Class.extend(b.Squarespace.DialogField, {
    initialize: function(a, d, e) {
      this.hideAnim = null;
      this._super(a, d, e);
      this.control = b.DB.TEXTAREA("field-textarea", {
        tabIndex: e.getNextTabIndex()
      });
      this.config.titleStyle || (this.config.titleStyle = "top");
      d[this.config.name] && this.control.set("value", d[this.config.name]);
      this.fieldTitleEl = this.config.title && "top" == this.config.titleStyle ? b.DB.DIV("field-lhs top", {
        html: this.config.title
      }) : null;
      this.html = b.DB.DIV("field-wrapper textarea-field-wrapper clear " + this.config.style, this.fieldTitleEl, b.DB.DIV("field-rhs", b.DB.DIV("field-input-wrapper", this.control), this.config.description ? b.DB.DIV("field-description-wrapper", b.DB.DIV("field-description", {
        html: this.config.description
      })) : null));
      this.config.height && this.control.setStyle("height", this.config.height + "px");
      this.panel.bodyEvents.push(b.on("keydown", function(a) {
        9 === a.keyCode && this.updateInlineTitle();
        a.stopPropagation()
      }, this.control, this));
      this.panel.bodyEvents.push(b.on("keyup", function(a) {
        if (c(a.keyCode) || 13 == a.keyCode) this.onDataChanged(a)
      }, this.control, this));
      this.panel.bodyEvents.push(b.on("focus", this.onFocus, this.control, this));
      this.panel.bodyEvents.push(b.on("blur", this.onBlur, this.control, this));
      this.value && this.setValue(d[this.config.name]);
      this.config.hide && this.html.setStyle("display", "none")
    },
    isEmpty: function() {
      return !this.control ? !1 : 0 === this.control.get("value").length
    },
    enable: function() {
      this.html.removeClass("disabled");
      this.control.set("disabled", "")
    },
    disable: function() {
      this.html.addClass("disabled");
      this.control.set("disabled", "true")
    },
    showInlineTitle: function() {
      null !== this.hideAnim && (this.hideAnim.stop(), this.hideAnim = null);
      this.inlineTitle || (this.inlineTitle = b.DB.DIV("inline-field-title textarea", {
        html: this.config.title
      }), this.currentXY = [38, 12], this.inlineTitle.setStyles({
        top: this.currentXY[1] + "px",
        left: this.currentXY[0] + "px",
        opacity: 1
      }), this.inlineTitle.event = b.on("click", this.focus, this.inlineTitle, this), b.one(this.html).append(this.inlineTitle))
    },
    hideInlineTitle: function() {
      this.inlineTitle && null === this.hideAnim && (this.hideAnim = this._anim({
        node: this.inlineTitle,
        from: {
          opacity: 1
        },
        to: {
          opacity: 0
        },
        duration: 0.05,
        easing: b.Easing.easeOutStrong
      }), this.hideAnim.on("end", function() {
        this.inlineTitle.event.detach();
        this.inlineTitle.remove();
        this.hideAnim = this.inlineTitle = null
      }, this), this.hideAnim.run())
    },
    updateInlineTitle: function() {
      "inline" == this.config.titleStyle && (this.isEmpty() ? this.showInlineTitle() : this.hideInlineTitle())
    },
    focus: function() {
      this.control.focus()
    },
    onDataChanged: function(a) {
      this.updateInlineTitle();
      this.panel.fire("datachange", this);
      this.clearError()
    },
    onBlur: function() {
      window.CONFIG_PANEL && window.CONFIG_PANEL.set("allowContextMenu", !1)
    },
    onFocus: function() {
      window.CONFIG_PANEL && window.CONFIG_PANEL.set("allowContextMenu", !0)
    },
    show: function() {
      "inline" == this.config.titleStyle && this.isEmpty() && this.showInlineTitle()
    },
    hide: function() {
      "inline" == this.config.titleStyle && this.hideInlineTitle()
    },
    getTakenHeight: function() {
      return this.config.verticalSpan ? this.html.get("offsetHeight") - this.control.get("offsetHeight") : this.html.get("offsetHeight")
    },
    getValue: function() {
      return this.control.get("value")
    },
    setValue: function(a) {
      this.control.set("value", a ? a : "");
      this.updateInlineTitle();
      this.clearError()
    }
  });
  b.Squarespace.DialogFieldGenerators.progress = Class.extend(b.Squarespace.DialogField, {
    initialize: function(a, d, c) {
      this._super(a, d, c);
      this.config = a;
      this.panel = c;
      this.control = b.DB.DIV("field-progress-inner", {
        html: "&nbsp;",
        style: "width: 0px;"
      });
      this.controlContainer = b.DB.DIV("field-progress-wrapper", this.control);
      this.html = b.DB.DIV("field-wrapper clear", this.config.title ? b.DB.DIV("field-lhs", {
        html: this.config.title
      }) : null, b.DB.DIV("field-rhs", this.controlContainer))
    },
    setMessage: function(a) {
      this.control.set("innerHTML", '<div class="text dialog-element">' + a + "</div>")
    },
    setProgress: function(a, d) {
      1 < a && (a = 1);
      0 > a && (a = 0);
      this.a = this._anim({
        node: this.control,
        to: {
          width: Math.round(a * this.controlContainer.get("offsetWidth") - 6)
        },
        duration: 0.25,
        easing: b.Easing.easeOutStrong
      });
      if (d) this.a.on("end", function() {
        this.control.set("innerHTML", '<div class="text dialog-element">' + d + "</div>");
        this.a = null
      }, this);
      else this.a.on("end", function() {
        this.control.set("innerHTML", '<div class="text dialog-element">' + Math.round(100 * a) + "%</div>");
        this.a = null
      }, this);
      this.a.run();
      this.previousPercentage = a
    },
    hide: function() {
      this.a && this.a.stop()
    }
  });
  b.Squarespace.DialogFieldGenerators.html = Class.extend(b.Squarespace.DialogField, {
    initialize: function(a, d, c) {
      this._super(a, d, c);
      this.html = ("string" == typeof this.config.html ? b.Node.create(this.config.html) : this.config.html.cloneNode(!0)).addClass("field-wrapper clear html-field" + ("name" in this.config ? " " + this.config.name : "") + ("className" in this.config ? " " + this.config.className : ""))
    },
    getNode: function() {
      return this.getWrapperNode()
    },
    getWrapperNode: function() {
      return this.html
    },
    getContent: function() {
      return this.html.getContent()
    },
    setContent: function(a) {
      this.html.setContent(a)
    }
  });
  b.Squarespace.DialogFieldGenerators.description = Class.extend(b.Squarespace.DialogField, {
    initialize: function(a, d, c) {
      this._super(a, d, c);
      this.config = a;
      this.panel = c;
      this.config.text || (this.config.text = "");
      this.html = b.DB.DIV("field-wrapper clear", b.DB.DIV("custom-field-description " + (this.config.className ? this.config.className : ""), {
        html: this.config.title ? '<div class="title">' + this.config.title + '</div><div class="text">' + this.config.text + "</div>" : this.config.text,
        style: (this.config.width && 1 < this.config.width ? "width: " + this.config.width + "px;" : "") + (this.config.padding ? "padding-top: " + this.config.padding[0] + "px;" : "") + (this.config.padding ? "padding-bottom: " + this.config.padding[1] + "px;" : "")
      }))
    },
    setTitle: function(a) {
      this.html.one(".title").setContent(a)
    },
    setText: function(a) {
      a = this.config.title ? '<div class="title">' + this.config.title + '</div><div class="text">' + a + "</div>" : a;
      this.html.one(".custom-field-description").setContent(a)
    },
    onClick: function(a) {
      this.panel.fire("click", this.config.name)
    }
  });
  b.Squarespace.DialogFieldGenerators.link = Class.extend(b.Squarespace.DialogField, {
    initialize: function(a, d, c) {
      this._super(a, d, c);
      this.config = a;
      this.panel = c;
      this.html = b.DB.DIV("field-wrapper clear active custom-field-link " + (this.config.className ? this.config.className : ""), {
        html: "<span>" + (this.config.href ? '<a href="' + this.config.href + '">' + this.config.title + "</a>" : this.config.title) + "</span>"
      });
      a.align && this.html.setStyle("textAlign", a.align);
      this.html.on("click", this.onClick, this)
    },
    onClick: function(a) {
      this.panel.fire("link-" + this.config.name, this.config.name)
    }
  });
  b.Squarespace.DialogFieldGenerators["section-title"] = Class.extend(b.Squarespace.DialogField, {
    initialize: function(a, d, c) {
      this._super(a, d, c);
      this.config = a;
      this.panel = c;
      this.value = null;
      this.titleRhs = [];
      this.tabEls = {};
      if (this.config.tabs) for (a = 0; a < this.config.tabs.length; ++a) d = this.config.tabs[a], this.tabEls[d.name] = b.DB.DIV("tab", {
        html: d.title,
        data: {
          value: d.name
        }
      }), this.titleRhs.push(this.tabEls[d.name]),
      d.active && (this.tabEls[d.name].addClass("active"), this.activeEl = this.tabEls[d.name], this.value = d.name);
      this.html = b.DB.DIV("field-wrapper clear", b.DB.DIV("nothing", {
        style: (this.config.padding ? "padding-top: " + this.config.padding[0] + "px;" : "") + (this.config.padding ? "padding-bottom: " + this.config.padding[1] + "px;" : "")
      }, b.DB.DIV("field-section-title clear " + this.config.className, b.DB.DIV("lhs", {
        html: this.config.text
      }), b.DB.DIV("rhs", this.titleRhs))));
      this.panel.bodyEvents.push(b.on("click", this.onClick, this.html,
      this))
    },
    onClick: function(a) {
      a.target.hasClass("tab") && (this.activeEl && this.activeEl.removeClass("active"), a.target.addClass("active"), this.activeEl = a.target, this.value = b.DB.unpackData(a.target).value, this.config.linkFrame && this.panel.getField(this.config.linkFrame).show(this.value), this.panel.fire("datachange", this))
    },
    getValue: function() {
      return this.value
    }
  });
  b.Squarespace.DialogFieldGenerators.rating = Class.extend(b.Squarespace.DialogField, {
    initialize: function(a, d, c) {
      this._super(a, d, c);
      this.config = a;
      this.panel = c;
      this.active = d[this.config.name] ? !0 : !1;
      this.value = 0;
      this.control = b.DB.DIV("rating-slider");
      this.html = b.DB.DIV("field-wrapper", b.DB.DIV("rating-field-wrapper", b.DB.DIV("rating-element", b.DB.DIV("rating-overlay"), this.control)));
      this.panel.bodyEvents.push(b.on("mousedown", this.onClick, this.html, this));
      this.setValue(d.rating ? d.rating : 0)
    },
    onClick: function(a) {
      a = a.pageX - this.html.one(".rating-element").getX();
      for (var b = 0, c = 0; 11 > c; c++) if (0 < 13 * c - a) {
        b = c;
        break
      }
      this.value = b / 2;
      this.control.setStyle("width",
      13 * b)
    },
    setValue: function(a) {
      0 > a || 5 < a || (this.value = Math.round(2 * a) / 2, this.control.setStyle("width", 26 * this.value))
    },
    getValue: function() {
      return this.value ? this.value : 0
    }
  });
  b.Squarespace.DialogFieldGenerators["social-accounts"] = Class.extend(b.Squarespace.DialogField, {
    initialize: function(a, d, c) {
      this._super(a, d, c);
      this.config = a;
      this.panel = c;
      this.config.networks = "facebook twitter foursquare google tumblr email blogger linkedin reddit stumbleupon".split(" ");
      this.accountContainerEl = b.DB.DIV("social-accounts");
      this.html = b.DB.DIV("field-wrapper social-adder-wrapper", b.DB.DIV("header", b.DB.DIV("title", {
        html: "Social Links"
      }), b.DB.DIV("description", {
        html: "Add your social links."
      })), this.accountContainerEl);
      this.html.append(this.chooser);
      this.panel.bodyEvents.push(b.on("click", this.onClick, this.html, this));
      this.panel.bodyEvents.push(b.on("keyup", this.onKeyUp, this.html, this));
      d[this.config.name] ? this.render(d[this.config.name]) : this.accountContainerEl.append(this.renderSocialAccount({
        type: this.config.networks[0],
        username: ""
      }))
    },
    onKeyUp: function(a) {
      this.panel.fire("datachange", this)
    },
    onClick: function(a) {
      var b = a.target,
        c = b.ancestor(".social-account", !0);
      !b.hasClass("logo") && (!b.hasClass("chooser-button") && this.activeEl) && this.hideChooser(this.activeEl);
      a.halt();
      if (b.hasClass("logo")) this.toggleChooser(c);
      else if (b.hasClass("plus")) this.getCount() >= this.config.maxCount || (this.showChooser(c.insert(this.renderSocialAccount({
        type: this.config.networks[0],
        username: ""
      }), "after").next()), this.panel.fire("datachange",
      this));
      else if (b.hasClass("minus")) c.remove(), 1 > this.getCount() && this.accountContainerEl.append(this.renderSocialAccount({
        type: this.config.networks[0],
        username: ""
      })), this.panel.fire("datachange", this);
      else if (b.hasClass("chooser-button")) {
        b = c.one(".logo");
        c.one(".chooser");
        this.clearSocial(b);
        for (var f = this.config.networks, g = 0; g < f.length; g++) a.target.hasClass(f[g]) && (b.addClass(f[g]), c.one(".type").set("text", f[g]));
        this.hideChooser(c);
        this.panel.fire("datachange", this)
      }
    },
    clearSocial: function(a) {
      for (var b = this.config.networks, c = 0; c < b.length; c++) console.log(b[c]), a.removeClass(b[c])
    },
    toggleChooser: function(a) {
      a.one(".logo").hasClass("active") ? this.hideChooser(a) : this.showChooser(a)
    },
    hideChooser: function(a) {
      a.one(".logo").removeClass("active");
      this.activeEl = null;
      this._anim({
        node: a.one(".chooser"),
        to: {
          width: 0
        },
        duration: 0.25,
        easing: b.Easing.easeOutStrong
      }).run()
    },
    showChooser: function(a) {
      var d = a.one(".chooser");
      b.all(".social-account .chooser").setStyle("width", "0");
      var c = this._anim({
        node: d,
        to: {
          width: 154
        },
        duration: 0.25,
        easing: b.Easing.easeOutStrong
      });
      this._subscribe(c, "end", function() {
        var a = this.panel.bodyEl,
          b = d.get("region").bottom,
          c = a.get("region").bottom;
        a.hasClass("scrollable") && b > c && a.set("scrollTop", a.get("scrollHeight"))
      }, this);
      c.run();
      a.one(".logo").addClass("active");
      this.activeEl = a
    },
    renderSocialAccount: function(a, d) {
      for (var c = b.DB.DIV("chooser-container"), f = this.config.networks, g = 0; g < f.length; g++) c.append(b.DB.DIV("chooser-button " + f[g]));
      return b.DB.DIV("social-account " + (d ? "hidden" : ""),
      b.DB.DIV("logo " + (a.type ? a.type : ""), b.DB.DIV("chooser", c)), b.DB.DIV("type", {
        html: a.type
      }), b.DB.INPUT("username", {
        value: a.username,
        placeholder: "Enter Username",
        maxlength: 30,
        spellcheck: !1,
        autocomplete: !1
      }), b.DB.DIV("buttons", b.DB.DIV("plus"), b.DB.DIV("minus")))
    },
    render: function(a) {
      this.accountContainerEl.set("innerHTML", "");
      if (a) for (var b = 0; b < a.length; b++) this.accountContainerEl.append(this.renderSocialAccount(a[b]))
    },
    getValue: function() {
      var a = [];
      this.accountContainerEl.all(".social-account").each(function(b) {
        b = {
          type: b.one(".type").get("text"),
          username: b.one("input").get("value").replace(" ", "")
        };
        0 < b.username.length && a.push(b)
      }, this);
      return a
    },
    setValue: function(a) {
      this.render(a)
    },
    getCount: function() {
      var a = 0;
      this.accountContainerEl.all(".social-account").each(function(b) {
        a++
      }, this);
      return a
    }
  });
  b.Squarespace.DialogFieldGenerators["date-picker"] = Class.extend(b.Squarespace.DialogField, {
    _name: "date-picker",
    _defaultOptions: {
      "yui3-calendar": {
        showPrevMonth: !0,
        showNextMonth: !0,
        width: "100%",
        height: "100%",
        selectionMode: "single"
      }
    },
    initialize: function(a, d, c) {
      this._super(a, d, c);
      this.config = b.merge(this._defaultOptions, a);
      this.calendarOptions = b.merge(this._defaultOptions["yui3-calendar"]);
      a["yui3-calendar"] && (this.calendarOptions = b.merge(this.calendarOptions, a["yui3-calendar"]));
      d[this.config.name] && (this.calendarOptions.date = new Date(d[this.config.name]));
      this.config.width && (this.calendarOptions.width = this.config.width);
      this.config.height && (this.calendarOptions.height = this.config.height);
      this.calendar = new b.Calendar(this.calendarOptions);
      this.html = b.DB.DIV("field-wrapper clear" + (this.config.className ? this.config.className : "") + " date-picker");
      this.calendarWrapper = b.DB.DIV("calendar-wrapper");
      this.html.append(this.calendarWrapper);
      this._subscribe(this.calendar, "dateClick", this.onSelectionChange);
      this.value = d[this.config.name] || null;
      this.calendar.selectDates([new Date(this.value)]);
      this.render()
    },
    render: function() {
      this.calendar.render(this.calendarWrapper)
    },
    focus: function() {
      this.calendar.focus()
    },
    enable: function() {
      this.html.removeClass("disabled");
      this.calendar.enable()
    },
    disable: function() {
      this.html.addClass("disabled");
      this.calendar.disable()
    },
    getValue: function() {
      return this.value ? this.value.valueOf() : null
    },
    onSelectionChange: function(a) {
      this.value = a.date
    },
    setValue: function(a) {
      this._super(a);
      b.Lang.isValue(a) && (this.calendar.deselectDates(), this.calendar.selectDates([this._shiftToWebsiteTimeZone(a)]))
    },
    _destroy: function() {
      this.calendar.destroy()
    },
    _shiftToWebsiteTimeZone: function(a) {
      a = new Date(a);
      var d = b.Squarespace.DateUtils.getTimeOffsetToWebsiteTimezone(a);
      a.setMinutes(a.getMinutes() + d);
      return a
    },
    _shiftFromWebsiteTimeZone: function(a) {
      a = new Date(a);
      var d = b.Squarespace.DateUtils.getTimeOffsetToWebsiteTimezone(a);
      a.setMinutes(a.getMinutes() - d);
      return a
    }
  });
  b.Squarespace.DialogFieldGenerators["datetime-picker"] = Class.extend(b.Squarespace.DialogFieldGenerators["date-picker"], {
    _name: "datetime-picker",
    initialize: function(a, d, c) {
      this._super(b.merge({
        dateFormat: "%A, %B %d %Y at %I:%M%p"
      }, a), d, c);
      d[a.name] ? this.setValue(new Date(d[a.name])) : this.config.clearable ? this.setValue(null) : this.setValue(new Date)
    },
    render: function() {
      this._renderPicker();
      this._renderLabel()
    },
    _renderPicker: function() {
      this.html = b.DB.DIV("field-wrapper clear " + (this.config.className ? this.config.className : "") + " datetime-picker-field");
      this.calendarWrapper = b.DB.DIV("calendar-wrapper");
      var a = new Date(this.value);
      this.hourControl = b.DB.INPUT("time-control hour", {
        maxlength: 2,
        style: "text-align: right;"
      });
      this.minuteControl = b.DB.INPUT("time-control minute", {
        maxlength: 2,
        style: "text-align: left;"
      });
      this.ampmControl = b.DB.INPUT("time-control ampm", {
        maxlength: 2,
        style: "text-align: left; width: 36px;"
      });
      this.timeWrapper = b.DB.DIV("time-wrapper", b.DB.DIV("time-wrapper-content", this.hourControl, b.DB.DIV("time-control", {
        html: ":",
        style: "display:inline-block;width: 5px; border: 0px"
      }), this.minuteControl, this.ampmControl));
      a && this.updateFlyout(a);
      this.calendar.render(this.calendarWrapper);
      this.dateFlyoutEvents = [b.on("keyup", this.onTimeFieldKeyUp, this.hourControl, this), b.on("keyup", this.onTimeFieldKeyUp,
      this.minuteControl, this), b.on("keydown", this.onTimeFieldKeyDown, this.hourControl, this), b.on("keydown", this.onTimeFieldKeyDown, this.minuteControl, this), b.on("keydown", this.onTimeFieldKeyDown, this.ampmControl, this), b.on("blur", this.onTimeFieldBlur, this.hourControl, this), b.on("blur", this.onTimeFieldBlur, this.minuteControl, this), b.on("click", this.onTimeFieldClick, this.hourControl, this), b.on("click", this.onTimeFieldClick, this.minuteControl, this), b.on("click", this.onTimeFieldClick, this.ampmControl, this),
        b.on("resize", this.onResize, b.one(window), this)];
      this.errorFlyoutAnchor = this.html
    },
    updateFlyout: function(a) {
      this.hourControl.set("value", b.Lang.trim(b.Squarespace.DateUtils.dateFormat(a, {
        format: "%l"
      })));
      this.minuteControl.set("value", b.Squarespace.DateUtils.dateFormat(a, {
        format: "%M"
      }));
      this.ampmControl.set("value", b.Squarespace.DateUtils.dateFormat(a, {
        format: "%p"
      }));
      this.calendar.set("date", a)
    },
    _renderLabel: function() {
      this.label = b.DB.DIV("field-workflow-description", b.DB.SPAN("datetime-label", this.config.title + "&nbsp;"), b.DB.SPAN("date", "Feb 20 2012"));
      this.html.append(b.DB.DIV("date-picker-field", this.label));
      this.panel.bodyEvents.push(b.on("click", this.onDatetimeChangeRequest, this.label.one(".date"), this));
      if (this.config.clearable) {
        var a = b.DB.DIV("remove-date", {
          html: "&nbsp;"
        });
        this.label.append(a);
        a.on("click", this.onClearDate, this)
      }
    },
    onClearDate: function() {
      this.setValue(null)
    },
    setValue: function(a) {
      this._super(a);
      this.updateDisplay()
    },
    onDatetimeChangeRequest: function(a) {
      this.flyout || (a.target.hasClass("date") ? (this.html.addClass("date-picker-active"), this.openFlyout(), this.calendarWrapper.one(".yui3-calendar").setStyle("width", "auto"), this.refreshTimeField()) : this.closeFlyout())
    },
    onResize: function() {
      this.flyout && this.alignFlyout()
    },
    alignFlyout: function() {
      b.one(document.body).get("docScrollY");
      this.dateFlyoutWidth = this.flyout.get("offsetWidth");
      var a = this.label.getX() - (this.dateFlyoutWidth - this.label.get("offsetWidth")) / 2 - 15,
        d = this.label.getY() + this.label.get("offsetHeight") + 5;
      this.flyout.setXY([a, d])
    },
    openFlyout: function() {
      this.label.getX();
      this.flyout = b.DB.DIV("workflow-flyout tight", b.DB.DIV("workflow-flyout-content", b.DB.DIV("flyout-notch-select-top"), b.DB.DIV("workflow-flyout-options", {
        style: "position:relative; top:-1px;"
      }, b.DB.DIV("flyout-title", "Select a Date & Time"), b.DB.DIV("flyout-calendar", this.calendarWrapper, this.timeWrapper))));
      b.one(document.body).append(this.flyout);
      this.flyout.one(".flyout-notch-select-top").setStyles({
        marginLeft: 192
      });
      this.dateFlyoutHeight = this.flyout.get("offsetHeight");
      this.dateFlyoutWidth = this.flyout.get("offsetWidth");
      this.flyout.setStyles({
        width: "400px",
        height: this.dateFlyoutHeight + "px",
        opacity: 1
      });
      this.dateFlyoutContents = this.flyout.one(".workflow-flyout-content");
      this.dateFlyoutContents.setStyles({
        marginTop: -1 * this.dateFlyoutHeight,
        opacity: 0
      });
      this.alignFlyout();
      this._anim({
        node: this.dateFlyoutContents,
        to: {
          marginTop: 0,
          opacity: 1
        },
        duration: 0.2,
        easing: b.Easing.easeOutStrong
      }).run();
      this.panel.setActiveFlyout({
        constraintClasses: ".workflow-flyout, .calendar-wrapper",
        field: this
      })
    },
    refreshTimeField: function() {
      this.value ? (this.timeWrapper.removeClass("disabled"), this.hourControl.set("value", b.Squarespace.DateUtils.dateFormat(this.value, {
        format: "%I"
      })), this.minuteControl.set("value", b.Squarespace.DateUtils.dateFormat(this.value, {
        format: "%M"
      })), this.ampmControl.set("value", b.Squarespace.DateUtils.dateFormat(this.value, {
        format: "%p"
      }))) : (this.timeWrapper.addClass("disabled"), this.hourControl.set("value", "10"), this.minuteControl.set("value", "00"), this.ampmControl.set("value", "AM"))
    },
    updateDisplay: function() {
      this.label ? this.value ? (this.label.one(".date").setContent(b.Squarespace.DateUtils.dateFormat(this.value, {
        format: this.config.dateFormat
      })), this.html.removeClass("no-date")) : (this.label.one(".date").setContent("Not Set"), this.html.addClass("no-date")) : console.log("[datetime-picker] tried to update display but no label el was found")
    },
    closeFlyout: function() {
      if (this.flyout) {
        this.html.removeClass("date-picker-active");
        var a = this.flyout.get("offsetHeight");
        animation = this._anim({
          node: this.dateFlyoutContents,
          to: {
            marginTop: -1 * a,
            opacity: 0
          },
          duration: 0.2,
          easing: b.Easing.easeOutStrong
        });
        animation.on("end", function() {
          this.flyout.remove();
          this.dateFlyoutContents = this.flyout = null
        }, this);
        animation.run();
        this.panel.clearActiveFlyout()
      }
    },
    onTimeFieldKeyUp: function(a) {
      a.target.hasClass("hour") ? (a = parseInt(this.hourControl.get("value"), 10), (12 < a || 1 > a) && this.hourControl.set("value", this.prevVal)) : a.target.hasClass("minute") && (a = parseInt(this.minuteControl.get("value"), 10), (60 < a || 0 > a) && this.minuteControl.set("value",
      this.prevVal));
      this._updateTime()
    },
    onTimeFieldClick: function(a) {
      a.target.hasClass("hour") ? this.hourControl.select() : a.target.hasClass("minute") ? this.minuteControl.select() : a.target.hasClass("ampm") && this.ampmControl.select()
    },
    onTimeFieldBlur: function(a) {
      a.target.hasClass("hour") ? (a = this.hourControl.get("value"), 0 === a.indexOf("0") && this.hourControl.set("value", parseInt(a, 10)), a = parseInt(a, 10), (12 < a || 1 > a) && this.hourControl.set("value", 12)) : a.target.hasClass("minute") && (a = parseInt(this.minuteControl.get("value"),
      10), 0 <= a && 9 >= a && this.minuteControl.set("value", "0" + a), a = parseInt(a, 10), (60 < a || 0 > a) && this.minuteControl.set("value", 0));
      this._updateTime()
    },
    onTimeFieldKeyDown: function(a) {
      65 == a.keyCode ? (this.ampmControl.set("value", "AM"), a.halt(), this._updateTime()) : 80 == a.keyCode ? (this.ampmControl.set("value", "PM"), a.halt(), this._updateTime()) : 13 == a.keyCode ? this.closeFlyout() : a.target.hasClass("hour") ? 186 == a.keyCode ? (this.minuteControl.select(), a.halt()) : (57 < a.keyCode || 48 > a.keyCode) && (96 > a.keyCode || 105 < a.keyCode) && 9 != a.keyCode && 37 != a.keyCode && 39 != a.keyCode && 16 != a.keyCode && 46 != a.keyCode && 8 != a.keyCode ? a.halt() : this.prevVal = this.hourControl.get("value") : a.target.hasClass("minute") ? 32 == a.keyCode ? (this.ampmControl.select(), a.halt()) : (57 < a.keyCode || 48 > a.keyCode) && (96 > a.keyCode || 105 < a.keyCode) && 9 != a.keyCode && 37 != a.keyCode && 39 != a.keyCode && 16 != a.keyCode && 46 != a.keyCode && 8 != a.keyCode ? a.halt() : this.prevVal = this.minuteControl.get("value") : a.target.hasClass("ampm") && 9 != a.keyCode && a.halt()
    },
    _updateTime: function() {
      var a = parseInt(this.hourControl.get("value"),
      10),
        d = parseInt(this.minuteControl.get("value"), 10),
        c = this.ampmControl.get("value");
      if (!isNaN(a) && !isNaN(d)) {
        if (12 < a || 0 >= a) a = 12;
        if (60 < d || 0 > d) d = 0;
        12 == a && "AM" == c ? a = 0 : 12 != a && "PM" == c && (a += 12);
        c = new Date(this.value.getTime());
        a -= b.Lang.trim(b.Squarespace.DateUtils.dateFormat(c, {
          format: "%H"
        }));
        d -= b.Lang.trim(b.Squarespace.DateUtils.dateFormat(c, {
          format: "%M"
        }));
        c.setHours(c.getHours() + a);
        c.setMinutes(c.getMinutes() + d);
        this.setValue(c);
        this.updateDisplay()
      }
    },
    onSelectionChange: function(a) {
      if (this.label) {
        a = a.date;
        var b = this._shiftToWebsiteTimeZone(this.value.getTime());
        b.set({
          year: a.getFullYear(),
          month: a.getMonth(),
          day: a.getDate(),
          second: 0,
          millisecond: 0
        });
        this.setValue(this._shiftFromWebsiteTimeZone(b.getTime()));
        this.updateDisplay()
      }
    }
  });
  b.Squarespace.DialogFieldGenerators["textbox-datetime-picker"] = Class.extend(b.Squarespace.DialogFieldGenerators["datetime-picker"], {
    initialize: function(a, d, c) {
      this._super(a, d, c);
      b.Squarespace.EscManager.addTarget(this);
      a["yui3-calendar"] && (this.maximumDate = a["yui3-calendar"].maximumDate)
    },
    getValue: function() {
      return null !== this.value ? this.value.valueOf() : null
    },
    render: function() {
      this._renderPicker();
      this.label = b.Node.create('<input type = "text" class = "field-input date" spellcheck="false"/>');
      b.Lang.isValue(this.config.title) && (this.title = b.Node.create('<div class="title">' + this.config.title + "</div>"), this.html.append(this.title));
      this.html.append(this.label);
      this.panel.bodyEvents.push(b.on("click", this.onDatetimeChangeRequest, this.label, this));
      this.label.on("keyup", function(a) {
        c(a.keyCode) && (this.lastChangeTime = (new Date).valueOf(), b.later(500, this, this.onKeyup, [this.lastChangeTime]))
      }, this);
      this.label.on("blur", this.updateDisplay, this)
    },
    updateDisplay: function() {
      if (this.value) {
        var a = b.Squarespace.DateUtils.dateFormat(this.value, {
          format: "%D @ %I:%M %p"
        });
        this.label.set("value", a)
      } else this.label.set("value", "")
    },
    closeFlyout: function() {
      this._super();
      this.updateDisplay()
    },
    onKeyup: function(a) {
      a = new Date(this.label.get("value"));
      b.Lang.isNumber(a.valueOf()) && (this.value = a, this.calendar.deselectDates(),
      this.updateFlyout(a), this.calendar.selectDates([a]))
    },
    onSelectionChange: function(a) {
      a = a.date;
      var d = this.value ? new Date(this.value) : new Date;
      a && (a.setHours(d.getHours()), a.setMinutes(d.getMinutes()), b.DataType.Date.isGreater(a, this.maximumDate) ? this.setValue(d) : (this.value = a, this.label && ("" === this.label.get("value") || a.valueOf() !== d.valueOf()) && this.updateDisplay()))
    },
    _destroy: function() {
      this._super();
      b.Squarespace.EscManager.removeTarget(this)
    },
    close: function() {
      this.closeFlyout()
    }
  });
  b.Squarespace.DialogFieldGenerators.picker = Class.extend(b.Squarespace.DialogField, {
    _name: "Picker",
    initialize: function(a, d, c) {
      this._super(a, d, c);
      this.control = b.DB.DIV("field-input-wrapper select", {
        html: "test"
      });
      this.html = b.DB.DIV("field-wrapper picker clear");
      this.valueToNode = {};
      this.value = d[this.config.name]
    },
    render: function() {
      var a = b.DB.DIV("options-wrapper");
      this.config.groups || this.renderGroup(a, this.config.data);
      a.delegate("click", this.onClick, ".option", this);
      this.html.append(a);
      this.value && this.setValue(this.value)
    },
    onClick: function(a) {
      a = a.target.ancestor(".option", !0).getData("value");
      this.setValue(a)
    },
    setValue: function(a) {
      var b = this.getValue();
      this._super(a);
      this.panel.fire("datachange", this);
      this.isValidValue(a) && (b && this.getNode(b).removeClass("active"), this.getNode(a).addClass("active"));
      this.dialog.edited = !0
    },
    renderGroup: function(a, d) {
      b.Array.each(d, function(d) {
        var c = b.DB.DIV("option", {
          data: {
            value: d.value
          }
        }, b.DB.DIV("title", {
          html: d.title
        }), d.description ? b.DB.DIV("description", {
          html: d.description
        }) : null);
        this.registerNode(d.value,
        c);
        a.append(c)
      }, this);
      return a
    },
    show: function() {
      this.render()
    },
    registerNode: function(a, b) {
      this.valueToNode[a] = b
    },
    getNode: function(a) {
      return this.valueToNode[a]
    },
    isValidValue: function(a) {
      if (!this.config.groups) for (var b = 0; b < this.config.data.length; b++) if (this.config.data[b].value == a) return !0;
      return !1
    }
  });
  b.Squarespace.DialogFieldGenerators.email = Class.extend(b.Squarespace.DialogFieldGenerators.text, {
    initialize: function(a, b, c) {
      this._super(a, b, c)
    },
    isValid: function() {
      for (var a = this.control.get("value"),
      b = [this._validateExistence, this._validateFormat], c = !0, f = 0; f < b.length; f++) {
        var g = b[f],
          c = c && g.call(this, a);
        if (!c) break
      }
      return c
    },
    getErrors: function() {
      var a = this.control.get("value");
      return [this._validateFormat, this._validateExistence].map(function(b) {
        return b.call(this, a)
      }, this).filter(b.Lang.isString)
    },
    _validateExistence: function(a) {
      var b = !0;
      this.config.required && "" === a && (b = !1);
      return !b ? "Required field" : null
    },
    _validateFormat: function(a) {
      return !b.Squarespace.EmailUtils.isValid(a) ? "The email address you entered is invalid." : null
    }
  });
  b.Squarespace.DialogFieldGenerators.numeric = Class.extend(b.Squarespace.DialogFieldGenerators.text, {
    initialize: function(a, b, c) {
      this._super(a, b, c);
      void 0 === b[a.name] && void 0 !== this.config.defaultValue && this.setValue(Number(this.config.defaultValue))
    },
    didDataChange: function() {
      return this.initialData[this.getName()] != Number(this.getValue())
    },
    getErrors: function() {
      var a = this._super();
      return this.config.required && ("" === this.getValue().trim() || !b.Lang.isNumber(Number(this.getValue()))) ? ["This must be a numeric value."] : a
    }
  });
  b.Squarespace.DialogFieldGenerators.url = Class.extend(b.Squarespace.DialogFieldGenerators.text, {
    initialize: function(a, d, c) {
      this._super(a, d, c);
      this.panel = c;
      b.DB.DIV("internal-linker", b.DB.DIV("icon"))
    }
  })
}, "1.0.0", {
  requires: "node datatype-date node-focusmanager anim dd attribute slider datatable json widget node-event-simulate calendar squarespace-gizmo squarespace-dombuilder squarespace-debugger squarespace-toggle squarespace-node-flyout squarespace-structured-input squarespace-mailcheck squarespace-util squarespace-date-utils squarespace-dialog-fields".split(" ")
});
YUI.add("dd-constrain", function(b, c) {
  var a = b.DD.DDM,
    d = null,
    e = function() {
      this._lazyAddAttrs = !1;
      e.superclass.constructor.apply(this, arguments)
    };
  e.NAME = "ddConstrained";
  e.NS = "con";
  e.ATTRS = {
    host: {},
    stickX: {
      value: !1
    },
    stickY: {
      value: !1
    },
    tickX: {
      value: !1
    },
    tickY: {
      value: !1
    },
    tickXArray: {
      value: !1
    },
    tickYArray: {
      value: !1
    },
    gutter: {
      value: "0",
      setter: function(a) {
        return b.DD.DDM.cssSizestoObject(a)
      }
    },
    constrain: {
      value: "view",
      setter: function(a) {
        var d = b.one(a);
        d && (a = d);
        return a
      }
    },
    constrain2region: {
      setter: function(a) {
        return this.set("constrain",
        a)
      }
    },
    constrain2node: {
      setter: function(a) {
        return this.set("constrain", b.one(a))
      }
    },
    constrain2view: {
      setter: function() {
        return this.set("constrain", "view")
      }
    },
    cacheRegion: {
      value: !0
    }
  };
  d = {
    _lastTickXFired: null,
    _lastTickYFired: null,
    initializer: function() {
      this._createEvents();
      this._eventHandles = [this.get("host").on("drag:end", b.bind(this._handleEnd, this)), this.get("host").on("drag:start", b.bind(this._handleStart, this)), this.get("host").after("drag:align", b.bind(this.align, this)), this.get("host").after("drag:drag",
      b.bind(this.drag, this))]
    },
    destructor: function() {
      b.Array.each(this._eventHandles, function(a) {
        a.detach()
      });
      this._eventHandles.length = 0
    },
    _createEvents: function() {
      b.Array.each(["drag:tickAlignX", "drag:tickAlignY"], function(a) {
        this.publish(a, {
          type: a,
          emitFacade: !0,
          bubbles: !0,
          queuable: !1,
          prefix: "drag"
        })
      }, this)
    },
    _handleEnd: function() {
      this._lastTickXFired = this._lastTickYFired = null
    },
    _handleStart: function() {
      this.resetCache()
    },
    _regionCache: null,
    _cacheRegion: function() {
      this._regionCache = this.get("constrain").get("region")
    },
    resetCache: function() {
      this._regionCache = null
    },
    _getConstraint: function() {
      var a = this.get("constrain"),
        d = this.get("gutter"),
        c;
      a && (a instanceof b.Node ? (this._regionCache || (this._eventHandles.push(b.on("resize", b.bind(this._cacheRegion, this), b.config.win)), this._cacheRegion()), c = b.clone(this._regionCache), this.get("cacheRegion") || this.resetCache()) : b.Lang.isObject(a) && (c = b.clone(a)));
      if (!a || !c) a = "view";
      "view" === a && (c = this.get("host").get("dragNode").get("viewportRegion"));
      b.Object.each(d, function(a, b) {
        c[b] = "right" === b || "bottom" === b ? c[b] - a : c[b] + a
      });
      return c
    },
    getRegion: function(a) {
      var b = {}, d = null,
        c = null,
        c = this.get("host"),
        b = this._getConstraint();
      a && (d = c.get("dragNode").get("offsetHeight"), c = c.get("dragNode").get("offsetWidth"), b.right -= c, b.bottom -= d);
      return b
    },
    _checkRegion: function(a) {
      var b = this.getRegion(),
        d = this.get("host"),
        c = d.get("dragNode").get("offsetHeight"),
        d = d.get("dragNode").get("offsetWidth");
      a[1] > b.bottom - c && (a[1] = b.bottom - c);
      b.top > a[1] && (a[1] = b.top);
      a[0] > b.right - d && (a[0] = b.right - d);
      b.left > a[0] && (a[0] = b.left);
      return a
    },
    inRegion: function(a) {
      a = a || this.get("host").get("dragNode").getXY();
      var b = this._checkRegion([a[0], a[1]]),
        d = !1;
      a[0] === b[0] && a[1] === b[1] && (d = !0);
      return d
    },
    align: function() {
      var a = this.get("host"),
        b = [a.actXY[0], a.actXY[1]],
        d = this.getRegion(!0);
      this.get("stickX") && (b[1] = a.startXY[1] - a.deltaXY[1]);
      this.get("stickY") && (b[0] = a.startXY[0] - a.deltaXY[0]);
      d && (b = this._checkRegion(b));
      b = this._checkTicks(b, d);
      a.actXY = b
    },
    drag: function() {
      var a = this.get("host"),
        d = this.get("tickX"),
        c = this.get("tickY"),
        a = [a.actXY[0], a.actXY[1]];
      if ((b.Lang.isNumber(d) || this.get("tickXArray")) && this._lastTickXFired !== a[0]) this._tickAlignX(), this._lastTickXFired = a[0];
      if ((b.Lang.isNumber(c) || this.get("tickYArray")) && this._lastTickYFired !== a[1]) this._tickAlignY(), this._lastTickYFired = a[1]
    },
    _checkTicks: function(b, d) {
      var c = this.get("host"),
        e = c.startXY[0] - c.deltaXY[0],
        c = c.startXY[1] - c.deltaXY[1],
        l = this.get("tickX"),
        m = this.get("tickY");
      l && !this.get("tickXArray") && (b[0] = a._calcTicks(b[0], e, l, d.left, d.right));
      m && !this.get("tickYArray") && (b[1] = a._calcTicks(b[1], c, m, d.top, d.bottom));
      this.get("tickXArray") && (b[0] = a._calcTickArray(b[0], this.get("tickXArray"), d.left, d.right));
      this.get("tickYArray") && (b[1] = a._calcTickArray(b[1], this.get("tickYArray"), d.top, d.bottom));
      return b
    },
    _tickAlignX: function() {
      this.fire("drag:tickAlignX")
    },
    _tickAlignY: function() {
      this.fire("drag:tickAlignY")
    }
  };
  b.namespace("Plugin");
  b.extend(e, b.Base, d);
  b.Plugin.DDConstrained = e;
  b.mix(a, {
    _calcTicks: function(a, b, d, c, e) {
      var m = (a - b) / d,
        n = Math.floor(m),
        p = Math.ceil(m);
      if ((0 !== n || 0 !== p) && m >= n && m <= p) a = b + d * n, c && e && (a < c && (a = b + d * (n + 1)), a > e && (a = b + d * (n - 1)));
      return a
    },
    _calcTickArray: function(a, b, d, c) {
      var e = 0,
        m = b.length,
        n = 0,
        p;
      if (!b || 0 === b.length) return a;
      if (b[0] >= a) return b[0];
      for (e = 0; e < m; e++) if (n = e + 1, b[n] && b[n] >= a) return p = a - b[e], a = b[n] - a, n = a > p ? b[e] : b[n], d && c && n > c && (n = b[e] ? b[e] : b[m - 1]), n;
      return b[b.length - 1]
    }
  })
}, "3.17.2", {
  requires: ["dd-drag"]
});
YUI.add("dd-ddm-base", function(b, c) {
  var a = function() {
    a.superclass.constructor.apply(this, arguments)
  };
  a.NAME = "ddm";
  a.ATTRS = {
    dragCursor: {
      value: "move"
    },
    clickPixelThresh: {
      value: 3
    },
    clickTimeThresh: {
      value: 1E3
    },
    throttleTime: {
      value: -1
    },
    dragMode: {
      value: "point",
      setter: function(a) {
        this._setDragMode(a);
        return a
      }
    }
  };
  b.extend(a, b.Base, {
    _createPG: function() {},
    _active: null,
    _setDragMode: function(a) {
      null === a && (a = b.DD.DDM.get("dragMode"));
      switch (a) {
      case 1:
      case "intersect":
        return 1;
      case 2:
      case "strict":
        return 2
      }
      return 0
    },
    CSS_PREFIX: b.ClassNameManager.getClassName("dd"),
    _activateTargets: function() {},
    _drags: [],
    activeDrag: !1,
    _regDrag: function(a) {
      if (this.getDrag(a.get("node"))) return !1;
      this._active || this._setupListeners();
      this._drags.push(a);
      return !0
    },
    _unregDrag: function(a) {
      var c = [];
      b.Array.each(this._drags, function(b) {
        b !== a && (c[c.length] = b)
      });
      this._drags = c
    },
    _setupListeners: function() {
      this._createPG();
      this._active = !0;
      var a = b.one(b.config.doc);
      a.on("mousemove", b.throttle(b.bind(this._docMove, this), this.get("throttleTime")));
      a.on("mouseup", b.bind(this._end, this))
    },
    _start: function() {
      this.fire("ddm:start");
      this._startDrag()
    },
    _startDrag: function() {},
    _endDrag: function() {},
    _dropMove: function() {},
    _end: function() {
      this.activeDrag && (this._shimming = !1, this._endDrag(), this.fire("ddm:end"), this.activeDrag.end.call(this.activeDrag), this.activeDrag = null)
    },
    stopDrag: function() {
      this.activeDrag && this._end();
      return this
    },
    _shimming: !1,
    _docMove: function(a) {
      this._shimming || this._move(a)
    },
    _move: function(a) {
      this.activeDrag && (this.activeDrag._move.call(this.activeDrag,
      a), this._dropMove())
    },
    cssSizestoObject: function(a) {
      a = a.split(" ");
      switch (a.length) {
      case 1:
        a[1] = a[2] = a[3] = a[0];
        break;
      case 2:
        a[2] = a[0];
        a[3] = a[1];
        break;
      case 3:
        a[3] = a[1]
      }
      return {
        top: parseInt(a[0], 10),
        right: parseInt(a[1], 10),
        bottom: parseInt(a[2], 10),
        left: parseInt(a[3], 10)
      }
    },
    getDrag: function(a) {
      var c = !1,
        f = b.one(a);
      f instanceof b.Node && b.Array.each(this._drags, function(a) {
        f.compareTo(a.get("node")) && (c = a)
      });
      return c
    },
    swapPosition: function(a, c) {
      a = b.DD.DDM.getNode(a);
      c = b.DD.DDM.getNode(c);
      var f = a.getXY(),
        g = c.getXY();
      a.setXY(g);
      c.setXY(f);
      return a
    },
    getNode: function(a) {
      return a instanceof b.Node ? a : a = a && a.get ? b.Widget && a instanceof b.Widget ? a.get("boundingBox") : a.get("node") : b.one(a)
    },
    swapNode: function(a, c) {
      a = b.DD.DDM.getNode(a);
      c = b.DD.DDM.getNode(c);
      var f = c.get("parentNode"),
        g = c.get("nextSibling");
      g === a ? f.insertBefore(a, c) : c === a.get("nextSibling") ? f.insertBefore(c, a) : (a.get("parentNode").replaceChild(c, a), f.insertBefore(a, g));
      return a
    }
  });
  b.namespace("DD");
  b.DD.DDM = new a
}, "3.17.2", {
  requires: ["node", "base",
    "yui-throttle", "classnamemanager"]
});
YUI.add("datatable-table", function(b, c) {
  var a = b.Array,
    d = b.Lang,
    e = d.sub,
    f = d.isArray,
    g = d.isFunction;
  b.namespace("DataTable").TableView = b.Base.create("table", b.View, [], {
    CAPTION_TEMPLATE: '<caption class="{className}"></caption>',
    TABLE_TEMPLATE: '<table cellspacing="0" class="{className}"></table>',
    getCell: function() {
      return this.body && this.body.getCell && this.body.getCell.apply(this.body, arguments)
    },
    getClassName: function() {
      var c = this.host,
        d = c && c.constructor.NAME || this.constructor.NAME;
      return c && c.getClassName ? c.getClassName.apply(c, arguments) : b.ClassNameManager.getClassName.apply(b.ClassNameManager, [d].concat(a(arguments, 0, !0)))
    },
    getRecord: function() {
      return this.body && this.body.getRecord && this.body.getRecord.apply(this.body, arguments)
    },
    getRow: function() {
      return this.body && this.body.getRow && this.body.getRow.apply(this.body, arguments)
    },
    _afterSummaryChange: function(a) {
      this._uiSetSummary(a.newVal)
    },
    _afterCaptionChange: function(a) {
      this._uiSetCaption(a.newVal)
    },
    _afterWidthChange: function(a) {
      this._uiSetWidth(a.newVal)
    },
    _bindUI: function() {
      var a;
      this._eventHandles || (a = b.bind("_relayAttrChange", this), this._eventHandles = this.after({
        columnsChange: a,
        modelListChange: a,
        summaryChange: b.bind("_afterSummaryChange", this),
        captionChange: b.bind("_afterCaptionChange", this),
        widthChange: b.bind("_afterWidthChange", this)
      }))
    },
    _createTable: function() {
      return b.Node.create(e(this.TABLE_TEMPLATE, {
        className: this.getClassName("table")
      })).empty()
    },
    _defRenderBodyFn: function(a) {
      a.view.render()
    },
    _defRenderFooterFn: function(a) {
      a.view.render()
    },
    _defRenderHeaderFn: function(a) {
      a.view.render()
    },
    _defRenderTableFn: function(a) {
      var c = this.get("container"),
        d = this.getAttrs();
      this.tableNode || (this.tableNode = this._createTable());
      d.host = this.get("host") || this;
      d.table = this;
      d.container = this.tableNode;
      this._uiSetCaption(this.get("caption"));
      this._uiSetSummary(this.get("summary"));
      this._uiSetWidth(this.get("width"));
      if (this.head || a.headerView) this.head || (this.head = new a.headerView(b.merge(d, a.headerConfig))), this.fire("renderHeader", {
        view: this.head
      });
      if (this.foot || a.footerView) this.foot || (this.foot = new a.footerView(b.merge(d, a.footerConfig))), this.fire("renderFooter", {
        view: this.foot
      });
      d.columns = this.displayColumns;
      if (this.body || a.bodyView) this.body || (this.body = new a.bodyView(b.merge(d, a.bodyConfig))), this.fire("renderBody", {
        view: this.body
      });
      c.contains(this.tableNode) || c.append(this.tableNode);
      this._bindUI()
    },
    destructor: function() {
      this.head && this.head.destroy && this.head.destroy();
      delete this.head;
      this.foot && this.foot.destroy && this.foot.destroy();
      delete this.foot;
      this.body && this.body.destroy && this.body.destroy();
      delete this.body;
      this._eventHandles && (this._eventHandles.detach(), delete this._eventHandles);
      this.tableNode && this.tableNode.remove().destroy(!0)
    },
    _extractDisplayColumns: function() {
      function a(b) {
        var d, e, g;
        d = 0;
        for (e = b.length; d < e; ++d) g = b[d], f(g.children) ? a(g.children) : c.push(g)
      }
      var b = this.get("columns"),
        c = [];
      b && a(b);
      this.displayColumns = c
    },
    _initEvents: function() {
      this.publish({
        renderTable: {
          defaultFn: b.bind("_defRenderTableFn", this)
        },
        renderHeader: {
          defaultFn: b.bind("_defRenderHeaderFn",
          this)
        },
        renderBody: {
          defaultFn: b.bind("_defRenderBodyFn", this)
        },
        renderFooter: {
          defaultFn: b.bind("_defRenderFooterFn", this)
        }
      })
    },
    initializer: function(a) {
      this.host = a.host;
      this._initEvents();
      this._extractDisplayColumns();
      this.after("columnsChange", this._extractDisplayColumns, this)
    },
    _relayAttrChange: function(a) {
      var b = a.attrName;
      a = a.newVal;
      this.head && this.head.set(b, a);
      this.foot && this.foot.set(b, a);
      this.body && ("columns" === b && (a = this.displayColumns), this.body.set(b, a))
    },
    render: function() {
      this.get("container") && this.fire("renderTable", {
        headerView: this.get("headerView"),
        headerConfig: this.get("headerConfig"),
        bodyView: this.get("bodyView"),
        bodyConfig: this.get("bodyConfig"),
        footerView: this.get("footerView"),
        footerConfig: this.get("footerConfig")
      });
      return this
    },
    _uiSetCaption: function(a) {
      var c = this.tableNode,
        d = this.captionNode;
      a ? (d || (this.captionNode = d = b.Node.create(e(this.CAPTION_TEMPLATE, {
        className: this.getClassName("caption")
      })), c.prepend(this.captionNode)), d.setHTML(a)) : d && (d.remove(!0), delete this.captionNode)
    },
    _uiSetSummary: function(a) {
      a ? this.tableNode.setAttribute("summary", a) : this.tableNode.removeAttribute("summary")
    },
    _uiSetWidth: function(a) {
      var b = this.tableNode;
      b.setStyle("width", !a ? "" : this.get("container").get("offsetWidth") - (parseInt(b.getComputedStyle("borderLeftWidth"), 10) || 0) - (parseInt(b.getComputedStyle("borderLeftWidth"), 10) || 0) + "px");
      b.setStyle("width", a)
    },
    _validateView: function(a) {
      return g(a) && a.prototype.render
    }
  }, {
    ATTRS: {
      columns: {
        validator: f
      },
      width: {
        value: "",
        validator: d.isString
      },
      headerView: {
        value: b.DataTable.HeaderView,
        validator: "_validateView"
      },
      footerView: {
        validator: "_validateView"
      },
      bodyView: {
        value: b.DataTable.BodyView,
        validator: "_validateView"
      }
    }
  })
}, "3.17.2", {
  requires: ["datatable-core", "datatable-head", "datatable-body", "view", "classnamemanager"]
});
YUI.add("slider-value-range", function(b, c) {
  var a = Math.round;
  b.SliderValueRange = b.mix(function() {
    this._initSliderValueRange()
  }, {
    prototype: {
      _factor: 1,
      _initSliderValueRange: function() {},
      _bindValueLogic: function() {
        this.after({
          minChange: this._afterMinChange,
          maxChange: this._afterMaxChange,
          valueChange: this._afterValueChange
        })
      },
      _syncThumbPosition: function() {
        this._calculateFactor();
        this._setPosition(this.get("value"))
      },
      _calculateFactor: function() {
        var a = this.get("length"),
          b = this.thumb.getStyle(this._key.dim),
          c = this.get("min"),
          g = this.get("max"),
          a = parseFloat(a) || 150,
          b = parseFloat(b) || 15;
        this._factor = (g - c) / (a - b)
      },
      _defThumbMoveFn: function(a) {
        "set" !== a.source && this.set("value", this._offsetToValue(a.offset))
      },
      _offsetToValue: function(b) {
        b = a(b * this._factor) + this.get("min");
        return a(this._nearestValue(b))
      },
      _valueToOffset: function(b) {
        return a((b - this.get("min")) / this._factor)
      },
      getValue: function() {
        return this.get("value")
      },
      setValue: function(a) {
        return this.set("value", a)
      },
      _afterMinChange: function(a) {
        this._verifyValue();
        this._syncThumbPosition()
      },
      _afterMaxChange: function(a) {
        this._verifyValue();
        this._syncThumbPosition()
      },
      _verifyValue: function() {
        var a = this.get("value"),
          b = this._nearestValue(a);
        a !== b && this.set("value", b)
      },
      _afterValueChange: function(a) {
        this._setPosition(a.newVal, {
          source: "set"
        })
      },
      _setPosition: function(a, b) {
        this._uiMoveThumb(this._valueToOffset(a), b);
        this.thumb.set("aria-valuenow", a);
        this.thumb.set("aria-valuetext", a)
      },
      _validateNewMin: function(a) {
        return b.Lang.isNumber(a)
      },
      _validateNewMax: function(a) {
        return b.Lang.isNumber(a)
      },
      _setNewValue: function(c) {
        return b.Lang.isNumber(c) ? a(this._nearestValue(c)) : b.Attribute.INVALID_VALUE
      },
      _nearestValue: function(a) {
        var b = this.get("min"),
          c = this.get("max"),
          g;
        g = c > b ? c : b;
        b = c > b ? b : c;
        c = g;
        return a < b ? b : a > c ? c : a
      }
    },
    ATTRS: {
      min: {
        value: 0,
        validator: "_validateNewMin"
      },
      max: {
        value: 100,
        validator: "_validateNewMax"
      },
      minorStep: {
        value: 1
      },
      majorStep: {
        value: 10
      },
      value: {
        value: 0,
        setter: "_setNewValue"
      }
    }
  }, !0)
}, "3.17.2", {
  requires: ["slider-base"]
});
YUI.add("datatable-core", function(b, c) {
  var a = b.Attribute.INVALID_VALUE,
    d = b.Lang,
    e = d.isFunction,
    f = d.isObject,
    g = d.isArray,
    h = d.isString,
    k = d.isNumber,
    l = b.Array,
    m = b.Object.keys,
    d = b.namespace("DataTable").Core = function() {};
  d.ATTRS = {
    columns: {
      validator: g,
      setter: "_setColumns",
      getter: "_getColumns"
    },
    recordType: {
      getter: "_getRecordType",
      setter: "_setRecordType"
    },
    data: {
      valueFn: "_initData",
      setter: "_setData",
      lazyAdd: !1
    },
    recordset: {
      setter: "_setRecordset",
      getter: "_getRecordset",
      lazyAdd: !1
    },
    columnset: {
      setter: "_setColumnset",
      getter: "_getColumnset",
      lazyAdd: !1
    }
  };
  b.mix(d.prototype, {
    getColumn: function(a) {
      var b, c, d;
      if (b = f(a) && !g(a) ? a && a._node ? this.body.getColumn(a) : a : this.get("columns." + a)) return b;
      b = this.get("columns");
      if (k(a) || g(a)) {
        a = l(a);
        d = b;
        b = 0;
        for (c = a.length - 1; d && b < c; ++b) d = d[a[b]] && d[a[b]].children;
        return d && d[a[b]] || null
      }
      return null
    },
    getRecord: function(a) {
      var b = this.data.getById(a) || this.data.getByClientId(a);
      b || (k(a) && (b = this.data.item(a)), !b && (this.view && this.view.getRecord) && (b = this.view.getRecord.apply(this.view,
      arguments)));
      return b || null
    },
    _allowAdHocAttrs: !0,
    _afterColumnsChange: function(a) {
      this._setColumnMap(a.newVal)
    },
    _afterDataChange: function(a) {
      var b = a.newVal;
      this.data = a.newVal;
      !this.get("columns") && b.size() && this._initColumns()
    },
    _afterRecordTypeChange: function(a) {
      var b = this.data.toJSON();
      this.data.model = a.newVal;
      this.data.reset(b);
      !this.get("columns") && b && (b.length ? this._initColumns() : this.set("columns", m(a.newVal.ATTRS)))
    },
    _createRecordClass: function(a) {
      var c, d, e;
      if (g(a)) {
        c = {};
        d = 0;
        for (e = a.length; d < e; ++d) c[a[d]] = {}
      } else f(a) && (c = a);
      return b.Base.create("record", b.Model, [], null, {
        ATTRS: c
      })
    },
    destructor: function() {
      (new b.EventHandle(b.Object.values(this._eventHandles))).detach()
    },
    _getColumns: function(a, b) {
      return 8 < b.length ? this._columnMap : a
    },
    _getColumnset: function(a, b) {
      return this.get(b.replace(/^columnset/, "columns"))
    },
    _getRecordType: function(a) {
      return a || this.data && this.data.model
    },
    _initColumns: function() {
      var a = this.get("columns") || [],
        b;
      !a.length && this.data.size() && (b = this.data.item(0), b.toJSON && (b = b.toJSON()), this.set("columns", m(b)));
      this._setColumnMap(a)
    },
    _initCoreEvents: function() {
      this._eventHandles.coreAttrChanges = this.after({
        columnsChange: b.bind("_afterColumnsChange", this),
        recordTypeChange: b.bind("_afterRecordTypeChange", this),
        dataChange: b.bind("_afterDataChange", this)
      })
    },
    _initData: function() {
      var a = this.get("recordType"),
        c = new b.ModelList;
      a && (c.model = a);
      return c
    },
    _initDataProperty: function(a) {
      var c;
      this.data || (c = this.get("recordType"), this.data = a && a.each && a.toJSON ? a : new b.ModelList,
      c && (this.data.model = c), this.data.addTarget(this))
    },
    initializer: function(a) {
      var b = a.data,
        c = a.columns;
      this._initDataProperty(b);
      c || ((a = (a.recordType || a.data === this.data) && this.get("recordType")) ? c = m(a.ATTRS) : g(b) && b.length && (c = m(b[0])), c && this.set("columns", c));
      this._initColumns();
      this._eventHandles = {};
      this._initCoreEvents()
    },
    _setColumnMap: function(a) {
      function b(a) {
        var d, e, f, g;
        d = 0;
        for (e = a.length; d < e; ++d) f = a[d], (g = f.key) && !c[g] && (c[g] = f), c[f._id] = f, f.children && b(f.children)
      }
      var c = {};
      b(a);
      this._columnMap = c
    },
    _setColumns: function(a) {
      function c(a) {
        var b = {}, d, e, h;
        k.push(a);
        l.push(b);
        for (d in a) a.hasOwnProperty(d) && (e = a[d], g(e) ? b[d] = e.slice() : f(e, !0) ? (h = t(k, e), b[d] = -1 === h ? c(e) : l[h]) : b[d] = a[d]);
        return b
      }
      function d(a, f) {
        var k = [],
          q, n, l, u;
        q = 0;
        for (n = a.length; q < n; ++q) {
          k[q] = l = h(a[q]) ? {
            key: a[q]
          } : c(a[q]);
          u = b.stamp(l);
          l.id || (l.id = u);
          l.field && (l.name = l.field);
          f ? l._parent = f : delete l._parent;
          u = l;
          var t = l.name || l.key || l.id,
            t = t.replace(/\s+/, "-");
          e[t] ? t += e[t]++ : e[t] = 1;
          u._id = t;
          g(l.children) && (l.children = d(l.children, l))
        }
        return k
      }
      var e = {}, k = [],
        l = [],
        t = b.Array.indexOf;
      return a && d(a)
    },
    _setColumnset: function(b) {
      this.set("columns", b);
      return g(b) ? b : a
    },
    _setData: function(b) {
      null === b && (b = []);
      if (g(b)) this._initDataProperty(), this.data.reset(b, {
        silent: !0
      }), b = this.data;
      else if (!b || !b.each || !b.toJSON) b = a;
      return b
    },
    _setRecordset: function(a) {
      var c;
      a && (b.Recordset && a instanceof b.Recordset) && (c = [], a.each(function(a) {
        c.push(a.get("data"))
      }), a = c);
      this.set("data", a);
      return a
    },
    _setRecordType: function(b) {
      var c;
      e(b) && b.prototype.toJSON && b.prototype.setAttrs ? c = b : f(b) && (c = this._createRecordClass(b));
      return c || a
    }
  })
}, "3.17.2", {
  requires: ["escape", "model-list", "node-event-delegate"]
});
YUI.add("datatable-message", function(b, c) {
  var a;
  b.namespace("DataTable").Message = a = function() {};
  a.ATTRS = {
    showMessages: {
      value: !0,
      validator: b.Lang.isBoolean
    }
  };
  b.mix(a.prototype, {
    MESSAGE_TEMPLATE: '<tbody class="{className}"><tr><td class="{contentClass}" colspan="{colspan}"></td></tr></tbody>',
    hideMessage: function() {
      this.get("boundingBox").removeClass(this.getClassName("message", "visible"));
      return this
    },
    showMessage: function(a) {
      a = this.getString(a) || a;
      this._messageNode || this._initMessageNode();
      this.get("showMessages") && (a ? (this._messageNode.one("." + this.getClassName("message", "content")).setHTML(a), this.get("boundingBox").addClass(this.getClassName("message", "visible"))) : this.hideMessage());
      return this
    },
    _afterMessageColumnsChange: function() {
      var a;
      this._messageNode && (a = this._messageNode.one("." + this.getClassName("message", "content"))) && a.set("colSpan", this._displayColumns.length)
    },
    _afterMessageDataChange: function() {
      this._uiSetMessage()
    },
    _afterShowMessagesChange: function(a) {
      a.newVal ? this._uiSetMessage(a) : this._messageNode && (this.get("boundingBox").removeClass(this.getClassName("message", "visible")), this._messageNode.remove().destroy(!0), this._messageNode = null)
    },
    _bindMessageUI: function() {
      this.after(["dataChange", "*:add", "*:remove", "*:reset"], b.bind("_afterMessageDataChange", this));
      this.after("columnsChange", b.bind("_afterMessageColumnsChange", this));
      this.after("showMessagesChange", b.bind("_afterShowMessagesChange", this))
    },
    initializer: function() {
      this._initMessageStrings();
      this.get("showMessages") && this.after("table:renderBody",
      b.bind("_initMessageNode", this));
      this.after(b.bind("_bindMessageUI", this), this, "bindUI");
      this.after(b.bind("_syncMessageUI", this), this, "syncUI")
    },
    _initMessageNode: function() {
      this._messageNode || (this._messageNode = b.Node.create(b.Lang.sub(this.MESSAGE_TEMPLATE, {
        className: this.getClassName("message"),
        contentClass: this.getClassName("message", "content"),
        colspan: this._displayColumns.length || 1
      })), this._tableNode.insertBefore(this._messageNode, this._tbodyNode))
    },
    _initMessageStrings: function() {
      this.set("strings",
      b.mix(this.get("strings") || {}, b.Intl.get("datatable-message")))
    },
    _syncMessageUI: function() {
      this._uiSetMessage()
    },
    _uiSetMessage: function(a) {
      this.data.size() ? this.hideMessage() : this.showMessage(a && a.message || "emptyMessage")
    }
  });
  b.Lang.isFunction(b.DataTable) && b.Base.mix(b.DataTable, [a])
}, "3.17.2", {
  requires: ["datatable-base"],
  lang: ["en", "fr", "es", "hu", "it"],
  skinnable: !0
});
YUI.add("squarespace-checkout-shopping-cart-template", function(b) {
  var c = b.Handlebars;
  (function() {
    var a = c.template;
    (c.templates = c.templates || {})["checkout-shopping-cart.html"] = a(function(a, b, c, g, h) {
      this.compilerInfo = [4, ">= 1.0.0"];
      c = this.merge(c, a.helpers);
      h = h || {};
      g = this.escapeExpression;
      a = '<div class="loading-spinner"></div>\n\n<div class="title">Order Summary</div>\n\n<table>\n  <thead> \n    <tr>\n      <td class="item">Item</td>\n      <td class="quantity">Quantity</td>\n      <td class="price">Price</td>\n      <td class="remove"></td>\n    </tr>\n  </thead>\n  <tbody></tbody>\n</table>\n\n<div class="subtotal total">\n  <div class="price"></div>\n  <div class="label">Subtotal</div>\n</div>\n\n<div class="tax total">\n  <div class="price"></div>\n  <div class="label">Tax</div>\n</div>\n\n<div class="shipping total">\n  <div class="price"></div>\n  <div class="label">Shipping</div>\n</div>\n\n<div class="discounts total">\n  <div class="price"></div>\n  <div class="label">Discounts</div>\n</div>\n\n<div class="grand-total total">\n  <div class="price"></div>\n  <div class="label">Grand Total</div>\n</div>\n\n<div class="empty-message">\n  You have nothing in your shopping cart.&nbsp;\n  <a href="';
      (c = c.continueShoppingUrl) ? c = c.call(b, {
        hash: {},
        data: h
      }) : (c = b.continueShoppingUrl, c = "function" === typeof c ? c.apply(b) : c);
      return a += g(c) + '">Continue shopping</a>\n</div>\n'
    })
  })();
  b.Handlebars.registerPartial("checkout-shopping-cart.html".replace("/", "."), c.templates["checkout-shopping-cart.html"])
}, "1.0", {
  requires: ["handlebars-base"]
});
YUI.add("datatable-sort", function(b, c) {
  function a() {}
  var d = b.Lang,
    e = d.isBoolean,
    f = d.isString,
    g = d.isArray,
    h = d.isObject,
    k = b.Array,
    l = d.sub,
    m = {
      asc: 1,
      desc: -1,
      1: 1,
      "-1": -1
    };
  a.ATTRS = {
    sortable: {
      value: "auto",
      validator: "_validateSortable"
    },
    sortBy: {
      validator: "_validateSortBy",
      getter: "_getSortBy"
    },
    strings: {}
  };
  b.mix(a.prototype, {
    sort: function(a, c) {
      return this.fire("sort", b.merge(c || {}, {
        sortBy: a || this.get("sortBy")
      }))
    },
    SORTABLE_HEADER_TEMPLATE: '<div class="{className}" tabindex="0" unselectable="on"><span class="{indicatorClass}"></span></div>',
    toggleSort: function(a, c) {
      var d = this._sortBy,
        e = [],
        f, g, h;
      f = 0;
      for (g = d.length; f < g; ++f) h = {}, h[d[f]._id] = d[f].sortDir, e.push(h);
      if (a) {
        a = k(a);
        f = 0;
        for (g = a.length; f < g; ++f) {
          h = a[f];
          for (d = e.length - 1; 0 <= f; --f) if (e[d][h]) {
            e[d][h] *= -1;
            break
          }
        }
      } else {
        f = 0;
        for (g = e.length; f < g; ++f) for (h in e[f]) if (e[f].hasOwnProperty(h)) {
          e[f][h] *= -1;
          break
        }
      }
      return this.fire("sort", b.merge(c || {}, {
        sortBy: e
      }))
    },
    _afterSortByChange: function() {
      this._setSortBy();
      this._sortBy.length && (this.data.comparator || (this.data.comparator = this._sortComparator),
      this.data.sort())
    },
    _afterSortDataChange: function(a) {
      (a.prevVal !== a.newVal || a.newVal.hasOwnProperty("_compare")) && this._initSortFn()
    },
    _afterSortRecordChange: function(a) {
      var b, c;
      b = 0;
      for (c = this._sortBy.length; b < c; ++b) if (a.changed[this._sortBy[b].key]) {
        this.data.sort();
        break
      }
    },
    _bindSortUI: function() {
      var a = this._eventHandles;
      a.sortAttrs || (a.sortAttrs = this.after(["sortableChange", "sortByChange", "columnsChange"], b.bind("_uiSetSortable", this)));
      !a.sortUITrigger && this._theadNode && (a.sortUITrigger = this.delegate(["click",
        "keydown"], b.rbind("_onUITriggerSort", this), "." + this.getClassName("sortable", "column")))
    },
    _defSortFn: function(a) {
      this.set.apply(this, ["sortBy", a.sortBy].concat(a.details))
    },
    _getSortBy: function(a, b) {
      var c, d, e, f;
      b = b.slice(7);
      if ("state" === b) {
        c = [];
        d = 0;
        for (e = this._sortBy.length; d < e; ++d) f = this._sortBy[d], c.push({
          column: f._id,
          dir: f.sortDir
        });
        return {
          state: 1 === c.length ? c[0] : c
        }
      }
      return a
    },
    initializer: function() {
      var a = b.bind("_parseSortable", this);
      this._parseSortable();
      this._setSortBy();
      this._initSortFn();
      this._initSortStrings();
      this.after({
        "table:renderHeader": b.bind("_renderSortable", this),
        dataChange: b.bind("_afterSortDataChange", this),
        sortByChange: b.bind("_afterSortByChange", this),
        sortableChange: a,
        columnsChange: a
      });
      this.data.after(this.data.model.NAME + ":change", b.bind("_afterSortRecordChange", this));
      this.publish("sort", {
        defaultFn: b.bind("_defSortFn", this)
      })
    },
    _initSortFn: function() {
      var a = this;
      this.data._compare = function(b, c) {
        var d = 0,
          e, f, g, h, k;
        e = 0;
        for (f = a._sortBy.length; !d && e < f; ++e) g = a._sortBy[e], d = g.sortDir, h = g.caseSensitive,
        g.sortFn ? d = g.sortFn(b, c, - 1 === d) : (k = b.get(g.key) || "", g = c.get(g.key) || "", !h && ("string" === typeof k && "string" === typeof g) && (k = k.toLowerCase(), g = g.toLowerCase()), d = k > g ? d : k < g ? -d : 0);
        return d
      };
      this._sortBy.length ? (this.data.comparator = this._sortComparator, this.data.sort()) : delete this.data.comparator
    },
    _initSortStrings: function() {
      this.set("strings", b.mix(this.get("strings") || {}, b.Intl.get("datatable-sort")))
    },
    _onUITriggerSort: function(a) {
      var b = a.currentTarget.getAttribute("data-yui3-col-id"),
        c = b && this.getColumn(b),
        d, e, f;
      if (!("keydown" === a.type && 32 !== a.keyCode) && (a.preventDefault(), c)) {
        if (a.shiftKey) {
          d = this.get("sortBy") || [];
          e = 0;
          for (f = d.length; e < f; ++e) if (b === d[e] || 1 === Math.abs(d[e][b])) {
            h(d[e]) || (d[e] = {});
            d[e][b] = -(c.sortDir || 0) || 1;
            break
          }
          e >= f && d.push(c._id)
        } else d = [{}], d[0][b] = -(c.sortDir || 0) || 1;
        this.fire("sort", {
          originEvent: a,
          sortBy: d
        })
      }
    },
    _parseSortable: function() {
      var a = this.get("sortable"),
        b = [],
        c, d, e;
      if (g(a)) {
        c = 0;
        for (d = a.length; c < d; ++c) {
          e = a[c];
          if (!h(e, !0) || g(e)) e = this.getColumn(e);
          e && b.push(e)
        }
      } else if (a && (b = this._displayColumns.slice(), "auto" === a)) for (c = b.length - 1; 0 <= c; --c) b[c].sortable || b.splice(c, 1);
      this._sortable = b
    },
    _renderSortable: function() {
      this._uiSetSortable();
      this._bindSortUI()
    },
    _setSortBy: function() {
      var a = this._displayColumns,
        b = this.get("sortBy") || [],
        c = " " + this.getClassName("sorted"),
        d, e, f, g;
      this._sortBy = [];
      d = 0;
      for (e = a.length; d < e; ++d) f = a[d], delete f.sortDir, f.className && (f.className = f.className.replace(c, ""));
      b = k(b);
      d = 0;
      for (e = b.length; d < e; ++d) {
        f = b[d];
        a = 1;
        if (h(f)) for (f in g = f, g) if (g.hasOwnProperty(f)) {
          a = m[g[f]];
          break
        }
        f && (f = this.getColumn(f) || {
          _id: f,
          key: f
        }, f.sortDir = a, f.className || (f.className = ""), f.className += c, this._sortBy.push(f))
      }
    },
    _sortComparator: function(a) {
      return a
    },
    _uiSetSortable: function() {
      var a = this._sortable || [],
        c = this.getClassName("sortable", "column"),
        d = this.getClassName("sorted"),
        e = this.getClassName("sorted", "desc"),
        f = this.getClassName("sort", "liner"),
        g = this.getClassName("sort", "indicator"),
        h = {}, k, m, v, w, x, B;
      this.get("boundingBox").toggleClass(this.getClassName("sortable"), a.length);
      k = 0;
      for (m = a.length; k < m; ++k) h[a[k].id] = a[k];
      this._theadNode.all("." + c).each(function(a) {
        var b = h[a.get("id")],
          k = a.one("." + f);
        b ? b.sortDir || a.removeClass(d).removeClass(e) : (a.removeClass(c).removeClass(d).removeClass(e), k && k.replace(k.get("childNodes").toFrag()), (a = a.one("." + g)) && a.remove().destroy(!0))
      });
      k = 0;
      for (m = a.length; k < m; ++k) v = a[k], w = this._theadNode.one("#" + v.id), B = -1 === v.sortDir, w && (x = w.one("." + f), w.addClass(c), v.sortDir && (w.addClass(d), w.toggleClass(e, B), w.setAttribute("aria-sort", B ? "descending" : "ascending")), x || (x = b.Node.create(b.Lang.sub(this.SORTABLE_HEADER_TEMPLATE, {
        className: f,
        indicatorClass: g
      })), x.prepend(w.get("childNodes").toFrag()), w.append(x)), x = l(this.getString(1 === v.sortDir ? "reverseSortBy" : "sortBy"), {
        title: v.title || "",
        key: v.key || "",
        abbr: v.abbr || "",
        label: v.label || "",
        column: v.abbr || v.label || v.key || "column " + k
      }), w.setAttribute("title", x), w.setAttribute("aria-labelledby", v.id))
    },
    _validateSortable: function(a) {
      return "auto" === a || e(a) || g(a)
    },
    _validateSortBy: function(a) {
      return null === a || f(a) || h(a, !0) || g(a) && (f(a[0]) || h(a, !0))
    }
  }, !0);
  b.DataTable.Sortable = a;
  b.Base.mix(b.DataTable, [a])
}, "3.17.2", {
  requires: ["datatable-base"],
  lang: ["en", "fr", "es", "hu"],
  skinnable: !0
});
YUI.add("squarespace-toggle", function(b) {
  function c() {
    c.superclass.constructor.apply(this, arguments)
  }
  b.namespace("Squarespace.Widgets").ToggleButton = b.Base.create("toggleButton", b.Widget, [b.WidgetChild], {
    renderUI: function() {
      var a = this.get("contentBox"),
        c = this.get("active") ? "active" : "inactive";
      a.append(b.DB.DIV("switch-box-container animate " + c, b.DB.DIV("switch-box", b.DB.DIV("toggle-box", b.DB.DIV("togglebg", b.DB.DIV("on", {
        html: this.get("onLabel")
      }), b.DB.DIV("off", {
        html: this.get("offLabel")
      })))), b.DB.DIV("switch-shadow", {
        html: "&nbsp;"
      }), b.DB.DIV("handle-wrapper", b.DB.DIV("handle", {
        html: "&nbsp;"
      }))))
    },
    destructor: function() {
      b.detachAll(this.get("id") + "|*")
    },
    bindUI: function() {
      var a = this.get("contentBox"),
        b = this.get("id");
      a.one(".handle-wrapper").on(b + "|click", this._onClick, this);
      this.after(["activeChange", "enabledChange"], this.syncUI, this)
    },
    _onClick: function(a) {
      a.halt();
      this.get("enabled") && this.set("active", !this.get("active"))
    },
    syncUI: function() {
      var a = this.get("contentBox"),
        b = a.one(".switch-box-container"),
        a = a.all(".togglebg, .handle"),
        c = this.get("active");
      a.setStyles({
        marginLeft: null,
        left: null
      });
      c ? b.removeClass("inactive").addClass("active") : b.removeClass("active").addClass("inactive");
      b = this.get("boundingBox");
      this.get("enabled") ? b.removeClass("disabled").addClass("enabled") : b.removeClass("enabled").addClass("disabled")
    }
  }, {
    CSS_PREFIX: "sqs-toggle-button",
    ATTRS: {
      active: {
        value: !1
      },
      onLabel: {
        value: "On"
      },
      offLabel: {
        value: "Off"
      },
      enabled: {
        value: !0
      }
    }
  });
  c.NAME = "toggleDragPlugin";
  c.NS = "drag";
  b.extend(c, b.Plugin.Base, {
    initializer: function() {
      this._host = this.get("host");
      this.afterHostMethod("bindUI", this._afterBindUI)
    },
    destructor: function() {
      b.detachAll(this.get("id") + "|*");
      this._handleDrag && this._handleDrag.destroy()
    },
    _afterBindUI: function() {
      var a = this._host.get("contentBox");
      a.one(".switch-box");
      a.one(".handle-wrapper").get("region");
      var c = this.get("id");
      this._handleDrag = new b.DD.Drag({
        node: a.one(".handle")
      });
      this._handleDrag.plug(b.Plugin.DDProxy);
      this._handleDrag.on(c + "|drag:start", this._onDragStart, this);
      this._handleDrag.on(c + "|drag:drag", this._onDragDrag,
      this);
      this._handleDrag.on(c + "|drag:end", this._onDragEnd, this)
    },
    _onDragStart: function(a) {
      a = a.target;
      var c = this._host.get("contentBox").one(".handle-wrapper").get("region");
      a.hasPlugin("con") || a.plug(b.Plugin.DDConstrained, {
        constrain: c
      })
    },
    _onDragDrag: function(a) {
      var b = a.target,
        c = this._host.get("contentBox"),
        f = this.get("maxValue"),
        g = c.one(".handle-wrapper").getX(),
        b = Math.abs(g - b.lastXY[0]),
        b = Math.max(0, b),
        b = Math.min(f, b);
      c.one(".togglebg").setStyle("marginLeft", b);
      c.one(".handle").setStyle("left", a.target.get("dragNode").getX() - c.one(".handle-wrapper").getX())
    },
    _onDragEnd: function(a) {
      a = a.target;
      var b = a.get("node"),
        c = a.con.get("constrain"),
        c = Math.abs(c.left - c.right);
      a = Math.abs(a.nodeXY[0] - a.lastXY[0]) / (c - b.get("offsetWidth"));
      b = this._host.get("active");
      0.5 < a && this._host.set("active", !b);
      this._host.get("active") === b && this._host.syncUI()
    }
  }, {
    ATTRS: {
      resistance: {
        value: 10
      },
      maxValue: {
        value: 70
      }
    }
  });
  b.Squarespace.ToggleDrag = c
}, "1.0", {
  requires: ["widget", "widget-child", "dd-drag", "dd-constrain", "dd-proxy"]
});
YUI.add("dd-scroll", function(b, c) {
  var a = function() {
    a.superclass.constructor.apply(this, arguments)
  }, d, e;
  a.ATTRS = {
    parentScroll: {
      value: !1,
      setter: function(a) {
        return a ? a : !1
      }
    },
    buffer: {
      value: 30,
      validator: b.Lang.isNumber
    },
    scrollDelay: {
      value: 235,
      validator: b.Lang.isNumber
    },
    host: {
      value: null
    },
    windowScroll: {
      value: !1,
      validator: b.Lang.isBoolean
    },
    vertical: {
      value: !0,
      validator: b.Lang.isBoolean
    },
    horizontal: {
      value: !0,
      validator: b.Lang.isBoolean
    }
  };
  b.extend(a, b.Base, {
    _scrolling: null,
    _vpRegionCache: null,
    _dimCache: null,
    _scrollTimer: null,
    _getVPRegion: function() {
      var a = {}, a = this.get("parentScroll"),
        b = this.get("buffer"),
        c = this.get("windowScroll"),
        d = c ? [] : a.getXY(),
        e = c ? "winWidth" : "offsetWidth",
        m = c ? "winHeight" : "offsetHeight",
        n = c ? a.get("scrollTop") : d[1],
        c = c ? a.get("scrollLeft") : d[0];
      return this._vpRegionCache = a = {
        top: n + b,
        right: a.get(e) + c - b,
        bottom: a.get(m) + n - b,
        left: c + b
      }
    },
    initializer: function() {
      var a = this.get("host");
      a.after("drag:start", b.bind(this.start, this));
      a.after("drag:end", b.bind(this.end, this));
      a.on("drag:align", b.bind(this.align, this));
      b.one("win").on("scroll", b.bind(function() {
        this._vpRegionCache = null
      }, this))
    },
    _checkWinScroll: function(a) {
      var b = this._getVPRegion(),
        c = this.get("host"),
        d = this.get("windowScroll"),
        e = c.lastXY,
        m = !1,
        n = this.get("buffer"),
        p = this.get("parentScroll"),
        s = p.get("scrollTop"),
        r = p.get("scrollLeft"),
        q = e[1] + this._dimCache.h,
        u = e[1],
        t = e[0] + this._dimCache.w,
        z = e[0],
        y = u,
        v = z,
        w = s,
        x = r;
      this.get("horizontal") && (z <= b.left && (m = !0, v = e[0] - (d ? n : 0), x = r - n), t >= b.right && (m = !0, v = e[0] + (d ? n : 0), x = r + n));
      this.get("vertical") && (q >= b.bottom && (m = !0, y = e[1] + (d ? n : 0), w = s + n), u <= b.top && (m = !0, y = e[1] - (d ? n : 0), w = s - n));
      0 > w && (w = 0, y = e[1]);
      0 > x && (x = 0, v = e[0]);
      0 > y && (y = e[1]);
      0 > v && (v = e[0]);
      a ? (c.actXY = [v, y], c._alignNode([v, y], !0), c.actXY = [v, y], c._moveNode({
        node: p,
        top: w,
        left: x
      }), !w && !x && this._cancelScroll()) : m ? this._initScroll() : this._cancelScroll()
    },
    _initScroll: function() {
      this._cancelScroll();
      this._scrollTimer = b.Lang.later(this.get("scrollDelay"), this, this._checkWinScroll, [!0], !0)
    },
    _cancelScroll: function() {
      this._scrolling = !1;
      this._scrollTimer && (this._scrollTimer.cancel(),
      delete this._scrollTimer)
    },
    align: function(a) {
      this._scrolling && (this._cancelScroll(), a.preventDefault());
      this._scrolling || this._checkWinScroll()
    },
    _setDimCache: function() {
      var a = this.get("host").get("dragNode");
      this._dimCache = {
        h: a.get("offsetHeight"),
        w: a.get("offsetWidth")
      }
    },
    start: function() {
      this._setDimCache()
    },
    end: function() {
      this._dimCache = null;
      this._cancelScroll()
    }
  });
  b.namespace("Plugin");
  d = function() {
    d.superclass.constructor.apply(this, arguments)
  };
  d.ATTRS = b.merge(a.ATTRS, {
    windowScroll: {
      value: !0,
      setter: function(a) {
        a && this.set("parentScroll", b.one("win"));
        return a
      }
    }
  });
  b.extend(d, a, {
    initializer: function() {
      this.set("windowScroll", this.get("windowScroll"))
    }
  });
  d.NAME = d.NS = "winscroll";
  b.Plugin.DDWinScroll = d;
  e = function() {
    e.superclass.constructor.apply(this, arguments)
  };
  e.ATTRS = b.merge(a.ATTRS, {
    node: {
      value: !1,
      setter: function(a) {
        var c = b.one(a);
        c ? this.set("parentScroll", c) : !1 !== a && b.error("DDNodeScroll: Invalid Node Given: " + a);
        return c
      }
    }
  });
  b.extend(e, a, {
    initializer: function() {
      this.set("node", this.get("node"))
    }
  });
  e.NAME = e.NS = "nodescroll";
  b.Plugin.DDNodeScroll = e;
  b.DD.Scroll = a
}, "3.17.2", {
  requires: ["dd-drag"]
});
YUI.add("clickable-rail", function(b, c) {
  b.ClickableRail = b.mix(function() {
    this._initClickableRail()
  }, {
    prototype: {
      _initClickableRail: function() {
        this._evtGuid = this._evtGuid || b.guid() + "|";
        this.publish("railMouseDown", {
          defaultFn: this._defRailMouseDownFn
        });
        this.after("render", this._bindClickableRail);
        this.on("destroy", this._unbindClickableRail)
      },
      _bindClickableRail: function() {
        this._dd.addHandle(this.rail);
        this.rail.on(this._evtGuid + b.DD.Drag.START_EVENT, b.bind(this._onRailMouseDown, this))
      },
      _unbindClickableRail: function() {
        this.get("rendered") && this.get("contentBox").one("." + this.getClassName("rail")).detach(this.evtGuid + "*")
      },
      _onRailMouseDown: function(a) {
        this.get("clickableRail") && !this.get("disabled") && (this.fire("railMouseDown", {
          ev: a
        }), this.thumb.focus())
      },
      _defRailMouseDownFn: function(a) {
        a = a.ev;
        var b = this._resolveThumb(a),
          c = this._key.xyIndex,
          f = parseFloat(this.get("length"), 10),
          g, h;
        b && (g = b.get("dragNode"), h = parseFloat(g.getStyle(this._key.dim), 10), g = this._getThumbDestination(a, g), g = g[c] - this.rail.getXY()[c], g = Math.min(Math.max(g, 0), f - h), this._uiMoveThumb(g, {
          source: "rail"
        }), a.target = this.thumb.one("img") || this.thumb, b._handleMouseDownEvent(a))
      },
      _resolveThumb: function(a) {
        return this._dd
      },
      _getThumbDestination: function(a, b) {
        var c = b.get("offsetWidth"),
          f = b.get("offsetHeight");
        return [a.pageX - Math.round(c / 2), a.pageY - Math.round(f / 2)]
      }
    },
    ATTRS: {
      clickableRail: {
        value: !0,
        validator: b.Lang.isBoolean
      }
    }
  }, !0)
}, "3.17.2", {
  requires: ["slider-base"]
});
YUI.add("datatable-datasource", function(b, c) {
  function a() {
    a.superclass.constructor.apply(this, arguments)
  }
  b.mix(a, {
    NS: "datasource",
    NAME: "dataTableDataSource",
    ATTRS: {
      datasource: {
        setter: "_setDataSource"
      },
      initialRequest: {
        setter: "_setInitialRequest"
      }
    }
  });
  b.extend(a, b.Plugin.Base, {
    _setDataSource: function(a) {
      return a || new b.DataSource.Local(a)
    },
    _setInitialRequest: function() {},
    initializer: function(a) {
      b.Lang.isUndefined(a.initialRequest) || this.load({
        request: a.initialRequest
      })
    },
    load: function(a) {
      a = a || {};
      a.request = a.request || this.get("initialRequest");
      a.callback = a.callback || {
        success: b.bind(this.onDataReturnInitializeTable, this),
        failure: b.bind(this.onDataReturnInitializeTable, this),
        argument: this.get("host").get("state")
      };
      var c = a.datasource || this.get("datasource");
      c && c.sendRequest(a)
    },
    onDataReturnInitializeTable: function(a) {
      a = a.response && a.response.results || [];
      this.get("host").set("data", a)
    }
  });
  b.namespace("Plugin").DataTableDataSource = a
}, "3.17.2", {
  requires: ["datatable-base", "plugin", "datasource-local"]
});
YUI.add("squarespace-commerce-coupon-formatters", function(b) {
  b.namespace("Squarespace.CommerceCouponFormatters");
  b.Squarespace.CommerceCouponFormatters = {
    getStoreCollections: function() {
      return b.Squarespace.ContentCollectionCache.filter(function(c) {
        return (c = c.getTemplateConfiguration()) ? c.get("collectionType") == b.Squarespace.CollectionTypes.PRODUCT && !1 === c.get("folder") : !1
      })
    },
    getCouponSummary: function(c) {
      var a, d, e = b.clone(c),
        e = b.Squarespace.Commerce.normalizeAndCleanCouponData(e);
      e.minPrice = b.Squarespace.Commerce.moneyString(c.minPrice);
      e.productTitle = e.productTitle || "?";
      switch (c.type) {
      case b.Squarespace.CommerceCouponType.ALL_ORDERS:
        a = "Save <strong>{discountAmt}</strong> on any order.";
        d = "<strong>Free shipping on any order.</strong>";
        break;
      case b.Squarespace.CommerceCouponType.ORDERS_OVER:
        a = "Save <strong>{discountAmt}</strong> on any order over <strong>{minPrice}</strong>.";
        d = "<strong>Free shipping</strong> on any order over <strong>{minPrice}</strong>.";
        break;
      case b.Squarespace.CommerceCouponType.CATEGORIES:
        if (Static.IN_BACKEND) {
          var f = e.categories;
          f && 0 !== f.length ? (a = 'Save <strong>{discountAmt}</strong> on every item from categories:<ul class="category-list">', b.Array.each(f, function(b, c) {
            a = c !== f.length - 1 ? a + ("<li><strong>" + b + "</strong>, </li>") : a + ("<li><strong>" + b + "</strong></li>")
          }, this), a += "</ul>") : a = "Save <strong>{discountAmt}</strong> on every item from: <div>No category selections.</div>"
        } else a = "Save <strong>{discountAmt}</strong> on select products.";
        break;
      case b.Squarespace.CommerceCouponType.SINGLE_PRODUCT:
        a = "Save <strong>{discountAmt}</strong> on one <strong>{productTitle}</strong>.";
        break;
      default:
        throw "Unsupported coupon type";
      }
      switch (c.discountType) {
      case b.Squarespace.CommerceDiscountType.FLAT:
        e.discountAmt = b.Squarespace.Commerce.moneyString(c.discountAmt);
        break;
      case b.Squarespace.CommerceDiscountType.PERCENTAGE:
        e.discountAmt = c.discountAmt + "%";
        break;
      case b.Squarespace.CommerceDiscountType.FREE_SHIPPING:
        return b.Lang.sub(d, e)
      }
      return b.Lang.sub(a, e)
    }
  }
}, "1.0", {
  requires: ["squarespace-util", "squarespace-commerce-utils"]
});
YUI.add("squarespace-structured-input", function(b) {
  b.namespace("Squarespace.Widgets");
  var c = b.Squarespace.Widgets.StructuredInput = b.Base.create("structuredInput", b.Squarespace.Widgets.SSWidget, [b.WidgetChild], {
    destructor: function() {
      this._cursorBlinkInterval && this._cursorBlinkInterval.cancel();
      this._scrapeInterval && this._scrapeInterval.cancel();
      this._resumeBlinkTimer && this._resumeBlinkTimer.cancel();
      this.tooltip && this.tooltip.destroy()
    },
    renderUI: function() {
      c.superclass.renderUI.call(this);
      var a = this.get("contentBox");
      this.targetEl = a;
      this.contentEl = a.one(".structured-content");
      this.cursorEl = a.one(".cursor");
      this.helpEl = a.one(".help-tag");
      this.inputEl = a.one("input");
      this.get("tabIndex") && this.inputEl.set("tabIndex", this.get("tabIndex"));
      var b = parseInt(a.getStyle("paddingLeft"), 10),
        e = parseInt(a.getStyle("paddingTop"), 10),
        a = a.get("offsetHeight") - 2 * e;
      this.set("topPadding", e);
      this.set("leftPadding", b);
      this.set("lineHeight", a);
      this.cursorEl.setStyle("height", a + "px");
      this.cursorEl.setStyle("top", e + "px");
      this._configureToolTip();
      this._scrapeChars()
    },
    bindUI: function() {
      c.superclass.bindUI.call(this);
      var a = this.get("contentBox"),
        b = a.one("input");
      a.on("mousedown", this._startDragging, this);
      a.on("mousemove", this._dragTextInput, this);
      a.on("mouseup", this._endDragging, this);
      a.on("dblclick", this._selectAllInput, this);
      b.on("focus", this._handleOnFocus, this);
      b.on("blur", this._handleBlurEvent, this);
      b.on("keydown", this._handleTextInput, this)
    },
    syncUI: function() {
      c.superclass.syncUI.call(this);
      this._updateContent();
      this._updateCursor()
    },
    _handleTextInput: function(a) {
      var c = a.keyCode,
        e = this.hasSel();
      this._pauseBlink();
      if ((8 === c || 46 === c) && e) this.deleteSelection();
      else if (a.shiftKey && !e && (this.selEnd = this.selStart = this.cursorPos), 8 === c) this.deleteChars(this.cursorPos - 1, 1);
      else if (46 === c) this.deleteChars(this.cursorPos, 1);
      else if (65 === c && a.metaKey) this.selStart = this.cursorPos = 0, this.selEnd = this.length(), this._updateCursor();
      else if (38 === c) this.cursorPos = 0, a.shiftKey ? this.selEnd = this.cursorPos : e && this._clearSel(), this.movingLeft = !0, this._updateCursor();
      else if (40 === c) this.cursorPos = this.length(), a.shiftKey ? this.selEnd = this.cursorPos : e && this._clearSel(), this.movingLeft = !1, this._updateCursor();
      else if (37 === c) {
        if (a.shiftKey) {
          if (0 === this.selEnd) return;
          this.selEnd--
        } else if (e) this.cursorPos = Math.min(this.selStart, this.selEnd), this._clearSel();
        else {
          if (0 === this.cursorPos) return;
          this.cursorPos--
        }
        this.movingLeft = !0;
        this._updateCursor()
      } else if (39 === c) {
        if (a.shiftKey) {
          if (this.selEnd == this.length()) return;
          this.selEnd++
        } else if (e) this.cursorPos = Math.max(this.selStart, this.selEnd), this._clearSel();
        else {
          if (this.cursorPos == this.length()) return;
          this.cursorPos++
        }
        this.movingLeft = !1;
        this._updateCursor()
      } else b.later(1, this, this._scrapeChars)
    },
    _handleOnFocus: function(a) {
      this._startScrape();
      this._resumeBlink();
      this.hasSel() || this.cursorEl.setStyle("display", "block")
    },
    _handleBlurEvent: function(a) {
      this.mousepressed = !1;
      this._stopScrape();
      this._clearSel();
      this._pauseBlink();
      this._updateCursor();
      this.cursorEl.setStyle("display", "none")
    },
    _startDragging: function(a) {
      this.mousepressed = !0;
      this.focus(a);
      a.halt()
    },
    _dragTextInput: function(a) {
      this.mousepressed && (this.hasSel() || (this.selStart = this.cursorPos), a = a.clientX - this.targetEl.getXY()[0] - this.get("leftPadding") - this.get("fieldScroll"), this.selEnd = this.getDistanceToPos(a, "px"), this._updateCursor())
    },
    _endDragging: function(a) {
      this.mousepressed = !1
    },
    focus: function(a) {
      var b = this.length(),
        c = !1;
      a ? (a = a.clientX - this.targetEl.getXY()[0] - this.get("leftPadding") - this.get("fieldScroll"), this.cursorPos = this.getDistanceToPos(a, "px")) : this.cursorPos = b;
      !this.get("focused") && this.cursorPos == b && (this.selStart = b, this.selEnd = 0, c = this.get("selectAllOnFocus"));
      c || this._clearSel();
      this._updateCursor();
      this.inputEl.focus()
    },
    _stopScrape: function() {
      this._scrapeInterval && this._scrapeInterval.cancel()
    },
    _startScrape: function() {
      this._scrapeInterval = b.later(50, this, this._scrapeChars, null, !0)
    },
    _scrapeChars: function() {
      var a = this.inputEl.get("value");
      this.inputEl.set("value", "");
      b.Lang.isString(a) && 0 < a.length && (this.hasSel() && this.deleteSelection(), this.insertChars(a))
    },
    _pauseBlink: function() {
      this._cursorBlinkInterval && (this._cursorBlinkInterval.cancel(), this._cursorBlinkInterval = null, this.cursorEl.setStyle("visibility", "visible"), this._resumeBlinkTimer && this._resumeBlinkTimer.cancel(), this._resumeBlinkTimer = b.later(400, this, this._resumeBlink))
    },
    _resumeBlink: function() {
      this._cursorBlinkInterval || (this._resumeBlinkTimer && (this._resumeBlinkTimer.cancel(), this._resumeBlinkTimer = null), this._cursorBlinkInterval = b.later(600, this, this._blinkCursor, null, !0));
      this.cursorEl.setStyle("visibility", "visible")
    },
    _blinkCursor: function() {
      "hidden" == this.cursorEl.getStyle("visibility") ? this.cursorEl.setStyle("visibility", "visible") : this.cursorEl.setStyle("visibility", "hidden")
    },
    _updateCursor: function() {
      var a, c = this.get("leftPadding"),
        e = this.get("fieldScroll");
      if (this.hasSel()) {
        var f = this.getDistanceToPos(this.selStart, "cursor"),
          g = this.getDistanceToPos(this.selEnd, "cursor");
        a = this.movingLeft ? Math.min(f, g) : Math.max(f, g);
        this.selEl || (this.selEl = b.Node.create('<div class="sel"></div>'), this.targetEl.append(this.selEl), this.selEl.setStyles({
          top: parseInt(this.cursorEl.getStyle("top"),
          10) - 1 + "px",
          height: this.get("lineHeight") + 4 + "px"
        }), this.cursorEl.setStyle("display", "none"));
        this.selEl.setStyles({
          left: c + Math.min(f, g) + e + "px",
          width: Math.abs(f - g) + "px"
        })
      } else b.Lang.isUndefined(this.cursorPos) && (this.cursorPos = this.length()), a = f = this.getDistanceToPos(this.cursorPos, "cursor"), this.cursorEl.setStyle("left", c + f + "px"), this.selEl && (this.selEl.remove(), this.selEl = null, this.cursorEl.setStyle("display", "block"));
      c = this.targetEl.get("offsetWidth") - 64;
      f = !1;
      if (a + e - 1 > c) {
        for (; a + e - 1 > c;) e -= 60;
        f = !0
      } else if (a < -e) {
        for (; a < -e;) e += 60;
        f = !0
      }
      f && (this.contentEl.setStyle("margin-left", e + "px"), this.cursorEl.setStyle("margin-left", e + "px"));
      this.set("fieldScroll", e)
    },
    _updateContent: function() {
      var a = this.get("value"),
        c, e, f = "";
      if (0 === a.length) this.contentEl.setContent("&nbsp;");
      else {
        this.contentEl.setContent("");
        for (e = 0; e < a.length; ++e) if (this.isVariableAt(a, e)) {
          0 < f.length && (c = b.Node.create("<span>" + this.escapeBuffer(f) + "</span>"), this.contentEl.append(c), f = "");
          c = a[e] + a[e + 1];
          var g = this.get("variables")[c];
          g ? (c = b.Node.create('<span class="unbreakable">' + g.title + "</span>"), c.setAttribute("title", g.tip), this.contentEl.append(c)) : f += c;
          e += this.getVariableLengthAt(a, e) - 1
        } else f += a[e];
        0 < f.length && (c = b.Node.create("<span>" + this.escapeBuffer(f) + "</span>"), this.contentEl.append(c))
      }
    },
    insertChars: function(a) {
      var b = this._compressVariables(this.get("value")),
        c = b.substring(0, this.cursorPos),
        b = b.substring(this.cursorPos);
      this.set("value", this._uncompressVariables((c + a + b).replace(RegExp("\\s+", "g"), " ")));
      this.cursorPos += a.length;
      this.syncUI()
    },
    escapeBuffer: function(a) {
      return a.replace(RegExp("&", "g"), "&amp;").replace(RegExp("\\s+", "g"), "&nbsp;")
    },
    unescapeBuffer: function(a) {
      return a.replace(RegExp("&nbsp;", "g"), " ").replace(RegExp("&amp;", "g"), "&")
    },
    isVariableAt: function(a, c) {
      var e;
      if (!("%" != a[c] || c + 1 >= a.length)) return e = this.get("variables")[this.getVariableAt(a, c)], !b.Lang.isUndefined(e)
    },
    getVariableAt: function(a, b) {
      return a[b] + a[b + 1]
    },
    getVariableLengthAt: function(a, b) {
      return 2
    },
    _compressVariables: function(a) {
      var b = "",
        c, f;
      this.varStack = [];
      for (f = 0; f < a.length; ++f) {
        if (this.isVariableAt(a, f) && (c = this.getVariableAt(a, f), this.get("variables")[c])) {
          this.varStack.push(c);
          b += "\u0001";
          f += this.getVariableLengthAt(a, f) - 1;
          continue
        }
        b += a[f]
      }
      return b
    },
    _uncompressVariables: function(a) {
      var b = "",
        c = 0,
        f, g;
      f = 0;
      for (g = a.length; f < g; ++f) this.isVariableAt(a, f) && (this.cursorPos -= this.getVariableLengthAt(a, f) - 1), "\u0001" == a[f] ? (b += this.varStack[c], ++c) : b += a[f];
      delete this.varStack;
      return b
    },
    _varCount: function(a) {
      for (var b = 0, c = -1;;) {
        c = a.indexOf("\u0001",
        c);
        if (-1 == c) break;
        b++;
        c++
      }
      return b
    },
    deleteChars: function(a, b) {
      if (!(0 > a)) {
        var c = this._compressVariables(this.get("value")),
          f = c.substring(0, a),
          g = c.substring(a, a + b),
          c = c.substring(a + b),
          g = this._varCount(g);
        if (0 < g) for (var h = this._varCount(f); 0 < g;) this.varStack.splice(h, 1), g--;
        this.set("value", this._uncompressVariables(f + c));
        a < this.cursorPos && (this.cursorPos -= b);
        this.syncUI()
      }
    },
    deleteSelection: function() {
      this.deleteChars(Math.min(this.selStart, this.selEnd), Math.abs(this.selStart - this.selEnd));
      this.cursorPos = Math.min(this.selStart, this.selEnd);
      this._clearSel();
      this._updateCursor()
    },
    length: function() {
      return this._compressVariables(this.get("value")).length
    },
    getDistanceToPos: function(a, c) {
      if (0 === a) return 0;
      var e = b.Node.create('<span class="measure-span" style="visibility: hidden; left: -9999px"></span>'),
        f = 0,
        g, h;
      this.targetEl.append(e);
      this.contentEl.all("span").each(function(k) {
        if (!("cursor" == c && f == a || "px" == c && e.get("offsetWidth") >= a)) if (k.hasClass("unbreakable")) f++, e.append(b.Node.create('<span class="unbreakable">' + k.getContent() + "</span>")), "px" == c && e.get("offsetWidth") >= a && f--;
        else if (k = this.unescapeBuffer(k.getContent()), "cursor" == c) {
          var m = "";
          g = 0;
          for (h = k.length; g < h && !(m += k[g], f++, f == a); ++g);
          "" !== m && e.append(b.Node.create("<span>" + this.escapeBuffer(m) + "</span>"))
        } else {
          m = b.Node.create("<span></span>");
          e.append(m);
          g = 0;
          for (h = k.length; g < h; ++g) {
            var n = k[g];
            f++;
            " " == n && (n = "&nbsp;");
            m.setContent(m.getContent() + n);
            if (e.get("offsetWidth") >= a) {
              f--;
              break
            }
          }
        }
      }, this);
      var k = e.get("offsetWidth");
      e.remove();
      return "cursor" == c ? k : f
    },
    _configureToolTip: function() {
      var a = "This field accepts variables.  Type one of the codes below to activate a variable:<br/><br/>";
      b.Object.each(this.get("variables"), function(c, e) {
        a += b.Lang.sub("<strong>{key}</strong> &mdash; {tip}<br>", {
          key: e,
          tip: c.tip
        })
      });
      this.tooltip = new b.Squarespace.ToolTip({
        target: this.helpEl,
        title: "Formatting",
        dialogTooltip: !0,
        body: a.trim(),
        showTimeout: 200,
        width: 300,
        clickToShow: !0
      })
    },
    _clearSel: function() {
      delete this.selStart;
      delete this.selEnd
    },
    hasSel: function() {
      return !b.Lang.isUndefined(this.selStart) && !b.Lang.isUndefined(this.selStart)
    },
    _selectAllInput: function() {
      this.selStart = 0;
      this.selEnd = this.length();
      this._updateCursor()
    }
  }, {
    CSS_PREFIX: "sqs-structured-input",
    TEMPLATE: '<div><input class="structured-text-input" type="text"/><div class="structured-content"> </div><div class="help-tag"> </div><div class="cursor"></div></div>',
    ATTRS: {
      fieldScroll: {
        value: 0
      },
      lineHeight: {},
      leftPadding: {},
      topPadding: {},
      tabIndex: {},
      targetEl: {},
      selectAllOnFocus: {
        value: !1
      },
      strings: {},
      value: {
        value: "",
        validator: b.Lang.isString
      },
      variables: {}
    }
  })
}, "1.0", {
  requires: ["base", "squarespace-ss-widget", "widget-child", "squarespace-dombuilder", "squarespace-ui-base"]
});
YUI.add("squarespace-checkout-coupon-list-template", function(b) {
  var c = b.Handlebars;
  (function() {
    var a = c.template;
    (c.templates = c.templates || {})["checkout-coupon-list.html"] = a(function(a, b, c, g, h) {
      this.compilerInfo = [4, ">= 1.0.0"];
      this.merge(c, a.helpers);
      return '<div class="title">Coupons</div>\n\n<fieldset>\n\n  <div class="field">\n\n    <label>\n      Promo Code\n    </label>\n\n    <div class="codeline">\n      <div class="codeleft">\n        <input name="promoCode" class="field-element" type="text" placeholder="Promo Code" spellcheck="false" />\n       </div>\n\n      <!-- Redeem Button -->\n      <div class="button redeem-coupon">Redeem</div> \n    </div>\n\n    <!-- List -->\n    <div class="coupon-list"></div>\n\n    <div class="invalid-coupon-title">\n      The coupons below are no longer valid. This can occur if a coupon expires\n      or if you remove an item from your shopping cart.\n    </div>\n\n    <!-- No Longer Valid Coupons -->\n    <div class="invalid-coupon-list"></div>\n\n  </div>\n\n</fieldset>\n'
    })
  })();
  b.Handlebars.registerPartial("checkout-coupon-list.html".replace("/", "."), c.templates["checkout-coupon-list.html"])
}, "1.0", {
  requires: ["handlebars-base"]
});
YUI.add("squarespace-dialog", function(b) {
  b.namespace("Squarespace");
  b.Squarespace.OPEN_DIALOGS = [];
  b.Squarespace.DialogStates = {
    CLOSED: 1,
    EDITING: 2,
    LOADING: 3,
    CLOSING: 4,
    SAVING: 5,
    MOVING: 6
  };
  b.Squarespace.EditingDialog = Class.extend(b.Squarespace.ZombieGizmo, {
    _name: "EditingDialog",
    _events: {
      show: {},
      shown: {},
      aftershowanim: {},
      "align-to-anchor": {},
      "aligned-to-anchor": {},
      loading: {},
      "cancel-loading": {},
      "loading-ready": {},
      ready: {},
      drag: {},
      hide: {},
      hidden: {},
      close: {},
      closed: {},
      canceled: {},
      "cancel-clicked": {},
      "overlay-click": {},
      "show-save-overlay": {},
      "save-overlay-shown": {},
      "hide-save-overlay": {},
      "save-overlay-hidden": {},
      "render-anchor": {},
      datachange: {},
      datachanged: {},
      "send-requested": {
        emitFacade: !0
      },
      "remove-requested": {},
      "data-saved": {},
      "auto-save": {},
      "auto-save-requested": {
        emitFacade: !0
      },
      "allow-editing": {},
      "editing-allowed": {},
      "show-errors": {},
      "local-errors": {}
    },
    initialize: function(c) {
      this._super(c);
      this.setParams(c);
      this._setState("CLOSED");
      this.bodyEvents = [];
      this.buttonEvents = [];
      this.globalEvents = [];
      this.childDialogs = [];
      this.verticalFields = [];
      this.timers = [];
      this.fields = {};
      this.sections = {};
      this._noNameFields = [];
      this._setEdited(!1);
      this.NOTCH_WIDTH = 20;
      this.BUTTONS_BASE_IDX = 100;
      this.NOTCH_HEIGHT = 11;
      this.lastTabIndex = 1;
      this.on("datachange", this.onDataChange, this);
      this._debug = new b.Squarespace.Debugger({
        name: "EditingDialog",
        output: !1
      });
      this.publish("show", {
        prefix: "EditingDialog",
        broadcast: 2,
        emitFacade: !0
      });
      this.publish("dismiss", {
        prefix: "EditingDialog",
        broadcast: 2,
        emitFacade: !0
      });
      this.publish("tab-shown", {
        prefix: "EditingDialog",
        broadcast: 2,
        emitFacade: !0
      });
      this.publish("button-click", {
        prefix: "EditingDialog",
        broadcast: 2,
        emitFacade: !0,
        preventable: !1
      })
    },
    defaultOpts: {
      tabs: [],
      initialData: {},
      buttons: [],
      style: "standard",
      colorScheme: "light",
      buttonAlign: "right",
      position: "center",
      verticalHeight: "fixed",
      flyoutPointerDirection: "left",
      closingText: "Canceled...",
      savingText: "Saving...",
      top: 60,
      closeOthers: !0,
      disableTips: !0,
      closeable: !0,
      autoFocus: !0,
      discardChangesConfirmation: !0,
      overlay: !1,
      validateActiveTabOnly: !1,
      edgeMargin: 11,
      initialDataByReference: !1
    },
    getName: function() {
      return this.params.name
    },
    getInitialData: function() {
      return this.params.initialData
    },
    setParams: function(c) {
      if (c) {
        c.embedWithinEl && (c.closeable = !1, c.closeOthers = !1, c.style || (c.style = "transparent"), c.colorScheme || (c.colorScheme = "dark"), c.buttonAlign || (c.buttonAlign = "left"), c.top || (c.top = null));
        this.params = b.merge(this.defaultOpts, c);
        this.params.primaryTabs && (this.params.tabs = this.params.tabs.concat(this.params.primaryTabs), delete this.params.primaryTabs);
        this.params.secondaryTabs && (this.params.tabs = this.params.tabs.concat(this.params.secondaryTabs), delete this.params.secondaryTabs);
        if ((!this.params.tabs || 0 === this.params.tabs.length) && this.params.fields) this.params.tabs = [{
          fields: this.params.fields
        }];
        this.params.tabs[0].tabTitle || (this.params.tabs[0].tabTitle = "Item");
        this.params.tabs[0].name || (this.params.tabs[0].name = "item");
        "transparent" === this.params.style && (this.params.disableSaveOverlay = !0);
        !c.discardChangesConfirmation && 0 === this.params.buttons.length && (this.params.discardChangesConfirmation = !1);
        this.definitionChanged = !0
      }
    },
    show: function(c) {
      this.fire("show");
      this._debug.log("Showing", ["showParams", c], ["this.params", this.params]);
      this.moving && (this._setState("EDITING"), this.animation && this.animation.stop(), this.fire("cancel-loading"));
      if (!this.destroyTimer && !this._isState("LOADING") && !this._isState("CLOSING")) if (b.Squarespace.ToolTipManager && this.params.disableTips && b.Squarespace.ToolTipManager.disableTooltips(), this.anchorEl && this.anchorEl.removeClass("targeted"), c && this._setShowParams(c),
      this.params.parentDialog && this.params.parentDialog.addChildDialog(this), this._isState("CLOSED")) {
        this.definitionChanged = !1;
        if (this.params.closeOthers) {
          c = 0;
          for (var a = b.Squarespace.OPEN_DIALOGS.length; c < a; ++c) b.Squarespace.OPEN_DIALOGS[c].cancel()
        }
        b.Squarespace.OPEN_DIALOGS.push(this);
        b.one(document.body).addClass("dialog-open");
        this.fire("loading");
        this._setState("LOADING");
        this.timers.push(b.later(100, this, function() {
          this.params.closeable && b.Squarespace.EscManager.addTarget(this);
          this.globalEvents.push(b.on("resize",
          this.onResize, b.one(window), this))
        }));
        b.one(document).get("winWidth");
        b.one(document).get("winHeight");
        this._addTitleEl();
        this._addBodyEl();
        this.mainEl = b.Node.create('<div class="main-container"></div>').append(this.titleEl).append(this.bodyEl).append(this.controlsEl);
        c = "";
        this.params.disableStandardDialogWrapperClass || (c += "standard-dialog-wrapper ");
        c += "squarespace-managed-ui " + this.params.style + " " + this.params.colorScheme + " buttons-" + this.params.buttonAlign;
        this.params.name && (c += " dialog-" + b.Squarespace.Utils.slugify(this.params.name));
        this.el = b.Node.create('<div class="' + c + '"></div>').append(this.mainEl);
        this.bodyEvents.push(this.el.on("click", function(a) {
          this.cancelChildDialogs();
          this.fire("click", a)
        }, this));
        this.params.zIndex ? this.zIndex = this.params.zIndex : (b.Squarespace.DIALOG_ZINDEX_BASE += 10, this.zIndex = b.Squarespace.DIALOG_ZINDEX_BASE);
        this.el.setStyle("zIndex", this.zIndex);
        this.params.draggable && this.enableDragging();
        this.params.hidable && this._addHideEl();
        this.params.headerButton && this._addHeaderButtonEl();
        this.buttonHolder = b.Node.create("<div>").addClass("button-holder");
        this.autosaveEl = b.Node.create("<div>").addClass("autosave-state");
        this.controlsEl && (this.controlsEl.append(this.autosaveEl), this.controlsEl.append(this.buttonHolder));
        c = this.anchorEl ? this._positionWithAnchorEl() : this.params.embedWithinEl ? this._positionWithEmbededEl() : this._positionWithDefault();
        this.params.loadingState ? (c = this._anim({
          node: this.el,
          to: c[1] ? {
            opacity: 0.9,
            top: c[1]
          } : {
            opacity: 0.9
          },
          duration: 0.15,
          easing: b.Easing.easeOutStrong
        }), c.on("end", function() {
          this.fire("loading-ready")
        },
        this), c.run()) : this.dataReady();
        if ("full" == this.params.verticalHeight || "fit" == this.params.verticalHeight) this.el.setStyles({
          top: this.params.edgeMargin + "px",
          bottom: this.params.edgeMargin + "px"
        }), this.params.top = this.params.edgeMargin;
        this.params.overlay && (this.overlayEl = b.Node.create("<div>").addClass("dialog-screen-overlay"), this.overlayEl.setStyle("zIndex", this.zIndex - 1), b.one(document.body).append(this.overlayEl), JSTween.tween(this.overlayEl.getDOMNode(), {
          opacity: {
            start: 0,
            stop: 100 * this.params.overlay,
            time: 0,
            duration: 0.3,
            effect: "easeOut"
          }
        }), this.globalEvents.push(b.on("click", this.onOverlayClick, this.overlayEl, this)));
        this.moveIntoView();
        b.later(300, this, function() {
          this.fire("shown", this)
        })
      } else this.anchorEl && (this.anchorEl.addClass("targeted"), this._setState("LOADING"), this.moving = !0, this.updateTitle(), this.fire("loading"), this._updatePosition(!1), this.position && (this.animation = this._anim({
        node: this.el,
        to: {
          left: this.position.getX() + "px",
          top: this.position.getY() + "px"
        },
        duration: 0.25,
        easing: b.Easing.easeOutStrong
      }),
      this.animation.on("end", function() {
        this.fire("loading-ready");
        this.dataReady()
      }, this), this.animation.run()))
    },
    _setShowParams: function(b) {
      b.data && (this.data = b.data);
      b.anchor && (this.anchorEl = b.anchor);
      b.top && (this.params.top = b.top);
      b.flyoutPointerDirection && (this.params.flyoutPointerDirection = b.flyoutPointerDirection)
    },
    _addHeaderButtonEl: function() {
      var c = b.Node.create('<div class="header-button"></div>'),
        a = b.DB.INPUT({
          type: "button",
          value: this.params.headerButton.title,
          data: {
            type: this.params.headerButton.type
          }
        });
      b.on("click", this.params.headerButton.onclick, a, this);
      this.el.append(c.append(a))
    },
    _addBodyEl: function() {
      this.bodyEl = b.Node.create('<div class="body-block"></div>');
      0 !== this.params.buttons.length ? this.controlsEl = b.Node.create('<div class="controls-block"></div>') : this.bodyEl.addClass("bottom")
    },
    _addTitleEl: function() {
      var c = this.params.icon || this.params.tabs[0].icon,
        a = b.DB.DIV("icon", {
          style: "background-image: url(" + c + ");",
          data: {
            index: 0
          }
        });
      this.titleEl = b.Node.create('<div class="title-block loading"></div>');
      this.titlePrimaryTabEl = b.Node.create('<div class="icon-holder"></div>');
      this.titleTextEl = b.Node.create('<div class="text-holder"></div>');
      this.titleEl.append(this.titlePrimaryTabEl);
      this.titleEl.append(this.titleTextEl);
      this.tabsEl = b.Node.create('<div class="configuration-container-tabs"></div>');
      c && this.titlePrimaryTabEl.append(a);
      b.Lang.isNumber(this.currentTabIndex) || (this.currentTabIndex = 0);
      this.titleEl.append(this.tabsEl);
      this.updateTitle()
    },
    _addHideEl: function() {
      var c = b.Node.create('<div class="dialog-close"></div>');
      this.el.prepend(c);
      c.on("click", function() {
        this.fire("user-close");
        this.close()
      }, this)
    },
    _positionWithAnchorEl: function() {
      var c, a, d, e, f = this.params.width;
      this.el.setStyle("position", this.params.forcePosition || "absolute");
      this.anchorEl.addClass("targeted");
      "left" === this.params.flyoutPointerDirection || "right" === this.params.flyoutPointerDirection ? (this.el.setStyle("width", f + this.NOTCH_HEIGHT + "px"), this.mainEl.setStyle("width", f + "px"), this.el.addClass("flyout")) : "hidden" === this.params.flyoutPointerDirection ? (this.el.setStyle("width", f + "px"), this.mainEl.setStyle("width", f + "px")) : "top" === this.params.flyoutPointerDirection ? (this._initNotchEl("top"), this.el.insertBefore(this.notchEl, this.mainEl), d = this.anchorEl.get("offsetWidth"), a = this.anchorEl.get("offsetHeight"), c = this.anchorEl.getX(), a = this.anchorEl.getY() + a, f > d && (c -= (f - d) / 2), this.currentXY = [c, a - 3], this.notchEl.setStyle("marginLeft", (f - 30) / 2 + "px"), d = this.params.left || this.currentXY[0], e = this.params.top || this.currentXY[1], this.el.setStyles({
        left: d + "px",
        top: e + "px",
        width: f + "px"
      }), this.mainEl.setStyle("width", f + "px")) : "bottom" === this.params.flyoutPointerDirection ? (this._initNotchEl("top", "bottom"), this.el.append(this.notchEl), d = this.anchorEl.get("offsetWidth"), a = this.anchorEl.get("offsetHeight"), c = this.anchorEl.getX(), a = this.anchorEl.getY() + a, f > d && (c -= (f - d) / 2), this.currentXY = [c, a - 3], this.notchEl.setStyle("marginLeft", (f - 30) / 2 + "px"), d = this.params.left || this.currentXY[0], e = this.params.top || this.currentXY[1], this.el.setStyles({
        left: d + "px",
        top: e + "px",
        width: f + "px"
      }), this.mainEl.setStyle("width", f + "px")) : (this.currentXY = [this.anchorEl.getX() + (this.anchorEl.get("offsetWidth") - f) / 2, this.anchorEl.getY() + (this.anchorEl.get("offsetHeight") - this.params.height) / 2 - 15], this.el.setStyles({
        left: this.currentXY[0] + "px",
        top: this.currentXY[1] + "px",
        width: f + "px"
      }), this.el.setStyle(), this.mainEl.setStyle("width", f + "px"), a = this.currentXY[1]);
      b.one(b.config.doc.body).append(this.el);
      return [c, a]
    },
    _positionWithEmbededEl: function() {
      if (this.params.left || this.params.top) this.el.setStyle("position", "absolute"), this.params.top && this.el.setStyle("top", this.params.top + "px"), this.params.left && this.el.setStyle("left", this.params.left + "px");
      this.el.setStyle("width", this.params.width + "px");
      this.mainEl.setStyle("width", this.params.width + "px");
      this.params.embedWithinEl.append(this.el);
      return [0, 0]
    },
    _positionWithDefault: function() {
      var c, a = b.one(document).get("winWidth");
      b.one(document).get("winHeight");
      this.el.setStyle("position", "fixed");
      "right" === this.params.position ? (this.el.setStyles({
        right: this.params.edgeMargin + "px",
        top: "0px"
      }), 0 === this.params.edgeMargin && (this.titleEl.setStyle("border-radius", "0px"), this.controlsEl.setStyle("border-radius", "0px"))) : "undefined" !== typeof this.params.left ? this.currentXY = [this.params.left, this.params.top] : (this.currentXY = [(a - this.params.width) / 2, this.params.top], this.el.setStyle("position", "fixed"));
      this.currentXY && (this.el.setStyles({
        left: this.currentXY[0] + "px",
        top: this.currentXY[1] + "px"
      }), c = this.currentXY[1]);
      this.el.setStyle("width", this.params.width + "px");
      this.mainEl.setStyle("width",
      this.params.width + "px");
      b.one(document.body).append(this.el);
      return [void 0, c]
    },
    isOpen: function() {
      return this._isState("LOADING") || this._isState("EDITING")
    },
    disableBodyScroll: function() {
      this.oldBodyScroll = b.one("body").getStyle("overflow");
      b.one("body").setStyle("overflow", "hidden")
    },
    restoreBodyScroll: function() {
      this.hasOwnProperty("oldBodyScroll") && (this.oldBodyScroll ? b.one("body").setStyle("overflow", this.oldBodyScroll) : b.one("body").setStyle("overflow", null))
    },
    getAnchorEl: function() {
      return this.anchorEl
    },
    getChildDialogs: function() {
      return this.childDialogs
    },
    addChildDialog: function(b) {
      0 > this.childDialogs.indexOf(b) && (this.cancelChildDialogs(), this._debug.log("addChildDialog", b), this.childDialogs.push(b));
      b.params.parentDialog = this
    },
    removeChildDialog: function(c) {
      this.childDialogs = b.Array.filter(this.childDialogs, function(a) {
        return a !== c
      }, this)
    },
    enableDragging: function() {
      this.titleEl.setStyle("cursor", "move");
      this.dd = new b.DD.Drag({
        node: this.el
      });
      this.dd.on("drag:mouseDown", function(c) {
        b.Squarespace.Help && b.Squarespace.Help.active && c.halt();
        c.ev.target.ancestor(".body-block", !0) && c.halt()
      }, this);
      this.dd.on("drag:start", this.cancelChildDialogs, this);
      this.dd.on("drag:drag", function(b) {
        this.fire("drag", b)
      }, this);
      this.dd.on("drag:end", function(b) {
        this.moveIntoView()
      }, this)
    },
    temporaryHide: function(c) {
      this.fire("hide");
      b.Array.each(this.childDialogs, function(a) {
        a.temporaryHide()
      }, this);
      this.hideAnim && this.hideAnim.stop();
      this._hideAnimEvent && this._hideAnimEvent.detach();
      this.overlayHideAnim && this.overlayHideAnim.stop();
      this._overlayHideAnimEvent && this._overlayHideAnimEvent.detach();
      this.hideAnim = this._anim({
        node: this.el,
        to: {
          opacity: 0
        },
        duration: 0.35,
        easing: b.Easing.easeOutStrong
      });
      this.overlayEl && (this.overlayHideAnim = this._anim({
        node: this.overlayEl,
        to: {
          opacity: 0
        },
        duration: 0.35,
        easing: b.Easing.easeOutStrong
      }), this._overlayHideAnimEvent = this._subscribe(this.overlayHideAnim, "end", function() {
        this.overlayEl.setStyle("display", "none")
      }));
      this._hideAnimEvent = this._subscribe(this.hideAnim, "end", function() {
        this.el.setStyle("display", "none");
        this.fire("hidden", this)
      });
      c ? (this.el.setStyle("display", "none"), this.overlayEl && this.overlayEl.setStyle("display", "none"), this.fire("hidden", this)) : (this.hideAnim.run(), this.overlayEl && this.overlayHideAnim.run())
    },
    temporaryShow: function(c) {
      this.fire("show", this);
      c && this._setShowParams(c);
      this.hideAnim && this.hideAnim.stop();
      this._hideAnimEvent && this._hideAnimEvent.detach();
      this.overlayHideAnim && this.overlayHideAnim.stop();
      this.overlayEl && (this.overlayEl.setStyle("display", "block"), this.overlayHideAnim = this._anim({
        node: this.overlayEl,
        to: {
          opacity: this.params.overlay
        },
        duration: 0.35,
        easing: b.Easing.easeOutStrong
      }), this.overlayHideAnim.run());
      this.el.setStyle("display", "block");
      this.hideAnim = this._anim({
        node: this.el,
        to: {
          opacity: 1
        },
        duration: 0.35,
        easing: b.Easing.easeOutStrong
      });
      this._hideAnimEvent = this._subscribeOnce(this.hideAnim, "end", function() {
        this.fire("shown", this)
      });
      this.hideAnim.run();
      this._showChildren();
      this.moveIntoView()
    },
    _applyMethodToChildren: function() {
      var c = arguments[0],
        a = Array.prototype.slice.call(arguments,
        1);
      b.Array.each(this.childDialogs, function(b) {
        b[c].apply(b, a)
      }, this)
    },
    _showChildren: function() {
      this._applyMethodToChildren("temporaryShow")
    },
    _hideChildren: function() {
      this._applyMethodToChildren("temporaryHide")
    },
    cancelChildDialogs: function() {
      this._applyMethodToChildren("cancel");
      this.childDialogs = []
    },
    isVisible: function() {
      return !this._isState("CLOSED")
    },
    _updatePosition: function(c) {
      if (this.el && this.anchorEl) {
        var a = this.el.get("offsetHeight"),
          d = this.params.flyoutPointerDirection;
        if (!("top" === d || "bottom" === d)) {
          if ("centered" === d) {
            var e = this.anchorEl.getXY();
            this.position = new b.Squarespace.Position({
              x: e[0] + (this.anchorEl.get("offsetWidth") - this.params.width) / 2,
              y: e[1] + (this.anchorEl.get("offsetHeight") - a) / 2,
              w: this.params.width,
              h: a
            });
            this.position.nudgeFix()
          } else this.position = new b.Squarespace.Position({
            avoidElX: this.anchorEl,
            avoidElY: this.anchorEl,
            xdir: "right",
            ydir: "bottom",
            x: this.anchorEl.getX(),
            y: this.anchorEl.getY(),
            xo: 2,
            yo: 0,
            w: this.params.width + this.NOTCH_HEIGHT,
            h: a
          }), this.position.reflectFix();
          "hidden" !== d && "centered" !== d && this._reattachNotchEl();
          c && (this.el.setXY(this.position.getXY()), this._alignNotch(this.anchorEl))
        }
      }
    },
    _reattachNotchEl: function() {
      var b = "right" === this.position.xdir ? "left" : "right";
      this.notchEl ? (this.notchEl.set("className", "flyout-notch-" + b), this.notchEl.inDoc() && this.notchEl.remove()) : this._initNotchEl(b);
      "right" === this.position.xdir ? this.el.insertBefore(this.notchEl, this.mainEl) : this.el.append(this.notchEl)
    },
    _initNotchEl: function(c, a) {
      this.notchEl = b.Node.create('<div class="flyout-notch-' + c + '">&nbsp;</div>');
      b.Lang.isString(a) && this.notchEl.addClass(a)
    },
    _alignNotch: function(b) {
      if (this.notchEl) {
        var a = this.el.get("offsetHeight"),
          a = Math.max(a - 36, 220),
          d = 12,
          e = this.el.get("region");
        b = b.get("region");
        var f = b.top + b.height / 2;
        b.top > e.top && (d = f - e.top - 11, d = Math.min(a, Math.max(12, d)));
        this.notchEl.setStyle("marginTop", d + "px")
      }
    },
    onDataChange: function(b) {
      this.fire("render-anchor", this.getData());
      this.setEdited();
      this._debug.log("onDataChange");
      this.fire("datachanged")
    },
    onOverlayClick: function(b) {
      b.halt();
      this.fire("overlay-click");
      this.params.disableOverlayCancel || this.close()
    },
    setEdited: function() {
      this._isState("SAVING") || (this.params.discardChangesConfirmation && !this._onBeforeUnloadEvt && (this._onBeforeUnloadEvt = b.on("beforeunload", function(b) {
        this.isVisible() && (b.returnValue = "You have unsaved changes.")
      }, this)), this._setEdited(!0))
    },
    clearEdited: function() {
      this._onBeforeUnloadEvt && this._onBeforeUnloadEvt.detach();
      this._setEdited(!1)
    },
    _setEdited: function(b) {
      this.edited = !! b;
      this.editedSinceLastSave = !! b
    },
    getEditedSinceLastSave: function() {
      return this.editedSinceLastSave
    },
    getNextTabIndex: function() {
      return this.lastTabIndex++
    },
    getEdited: function() {
      return this.edited
    },
    isState: function(b) {
      return this.getState() === b
    },
    getState: function() {
      return this.state
    },
    setInitialData: function(c) {
      this.params.initialData = this.params.initialDataByReference ? c : b.clone(c, !0);
      this.fields && b.Object.each(this.fields, function(a, c) {
        this._isDialogField2(a) && (a.get("name") && !b.Lang.isUndefined(this.params.initialData[a.get("name")])) && (a.set("data", this.params.initialData[a.get("name")], {
          source: "setInitialData"
        }), a.setCurrentDataAsInitial())
      }, this)
    },
    getBodyHeight: function() {
      return this.params.height ? this.params.height : this._getFullHeight()
    },
    setActiveFlyout: function(c) {
      this.activeFlyout && this.activeFlyout.field.closeFlyout();
      b.later(10, this, function() {
        this.activeFlyout = c
      })
    },
    clearActiveFlyout: function() {
      this.activeFlyout = null
    },
    onResize: function(c) {
      if (!this.params.embedWithinEl) {
        c = b.one(document).get("winWidth");
        b.one(document).get("winHeight");
        if (!this.anchorEl && !this.params.draggable) {
          var a = {};
          if ("full" == this.params.verticalHeight || "fit" == this.params.verticalHeight) this.bodyHeight = this.getBodyHeight(), "fit" == this.params.verticalHeight && this.bodyHeight > this.observedHeight && (this.bodyHeight = this.observedHeight), this.bodyEl.setStyle("height", this.bodyHeight + "px"), this.bodyEl.all(".scrollable-body").each(function(a) {
            a.setStyles({
              height: this.bodyHeight - 30 + "px",
              paddingBottom: "30px"
            })
          }, this), this.resizeVerticalFields();
          "right" !== this.params.position && (this.hideErrors(), a.left = (c - this.params.width) / 2, a.top = this.params.top, this.params.left && (a.left = this.params.left));
          this.currentXY = [a.left, a.top];
          this.moveAnim && this.moveAnim.stop();
          this.moveAnim = this._anim({
            node: this.el,
            to: a,
            duration: 0.15,
            easing: b.Easing.easeOutStrong
          });
          this.moveAnim.run()
        }
        this.moveIntoView()
      }
    },
    moveIntoView: function(c) {
      if (this.params.draggable) {
        c && (!this.anchorEl && !this.position) && (c = !1);
        var a = {}, d = this.el.get("docScrollX"),
          e = this.el.get("docScrollY"),
          f = this.el.get("winWidth") + d,
          g = this.el.get("winHeight"),
          h = g + e,
          k = this.el.getY(),
          l = this.el.getX();
        k < e && (a.top = e);
        k + this.el.get("offsetHeight") > h && (a.top = h - this.el.get("offsetHeight"));
        l + this.el.get("offsetWidth") > f && (a.left = f - this.el.get("offsetWidth"));
        l < d && (a.left = d);
        c && (a.top = a.top ? Math.max(a.top, this.position.getY()) : this.position.getY());
        "fixed" === this.params.forcePosition && (b.Lang.isValue(a.top) && a.top > this.el.get("winHeight")) && (a.top = Math.max(0, g - this.el.get("offsetHeight")));
        0 !== b.Object.size(a) && this._anim({
          node: this.el,
          to: a,
          duration: 0.3,
          easing: b.Easing.easeOutStrong
        }).run();
        this.preferredHeight + 120 > g ? (this.bodyHeight = g - 120, this.bodyEl.setHeight(this.bodyHeight)) : this.bodyHeight != this.preferredHeight && (this.bodyHeight = this.preferredHeight, this.bodyEl.setHeight(this.preferredHeight))
      }
    },
    scrollIntoView: function(b) {
      this.el.scrollIntoView(b)
    },
    showErrors: function(c) {
      this.fire("show-errors", c);
      this.allowEditing();
      this.currentErrors = c;
      this.errorCount = b.Object.size(c);
      this.errorsByTab = {};
      b.Object.each(c, function(a, b) {
        var c = this.getField(b);
        c || console.error("Server error returned for a missing dialog field: " + b);
        c.tab.tabNavigationObj && c.tab.tabNavigationObj.addClass("error");
        this.errorsByTab[c.tab.name] ? this.errorsByTab[c.tab.name]++ : this.errorsByTab[c.tab.name] = 1
      }, this);
      this.activateErrors()
    },
    clearError: function(b) {
      this.currentErrors && this.currentErrors[b.getName()] && (this.errorsByTab[b.tab.name]--, delete this.currentErrors[b.getName()], this.errorCount--, b.tab.tabNavigationObj && 0 === this.errorsByTab[b.tab.name] && b.tab.tabNavigationObj.removeClass("error"))
    },
    activateErrors: function() {
      if (this.errorCount) {
        var c = null;
        b.Array.each(this.currentTab.tabFields, function(a) {
          this.currentErrors[a.getName()] && (c || (c = a, c.scrollIntoView()), a.showError(this.currentErrors[a.getName()]))
        }, this);
        c && b.Lang.isFunction(c.focus) && c.focus()
      }
    },
    hideErrors: function() {
      this.currentTab && b.Array.each(this.currentTab.tabFields || [], function(c) {
        b.Lang.isFunction(c.hideError) && c.hideError()
      }, this)
    },
    updateTitle: function(c) {
      var a = this.params.tabs[this.currentTabIndex];
      a && (this.currentTab && a.title != this.currentTab.title) && (this.currentTab.title = a.title);
      b.Lang.isUndefined(c) && (c = this.state);
      var a = this.currentTab && this.currentTab.title ? this.currentTab.title : this.params.title,
        d = "";
      this.params.subtext && (d = '<div class="title-subtext">' + this.params.subtext + "</div>");
      switch (c) {
      case b.Squarespace.DialogStates.LOADING:
        this.params.loadingText && this.setTitleHtml('<div class="title-text">' + this.params.loadingText + "</div>" + d);
        break;
      case b.Squarespace.DialogStates.EDITING:
        a && this.setTitleHtml('<div class="title-text">' + a + "</div>" + d);
        break;
      case b.Squarespace.DialogStates.SAVING:
        this.params.savingText && this.setTitleHtml('<div class="title-text">' + this.params.savingText + "</div>" + d)
      }
    },
    setTitleHtml: function(b) {
      this.titleTextEl.setHTML(b)
    },
    setData: function(c) {
      b.Array.each(this.currentTab.tabFields, function(a, d) {
        if (a.setValue) {
          var e = c[a.getName()];
          a.setValue(b.Lang.isValue(e) ? e : null);
          this.fire("datachange", a)
        }
      }, this)
    },
    focusTab: function() {
      this.currentTab && (this.fire("tab-focused", {
        tabName: this.currentTab.name
      }), b.Array.some(this.currentTab.tabFields,

      function(b, a) {
        var d = !(!b.setValue || !b.focus),
          e = this._isDialogField2(b),
          f = !(!e || !b.get("focusable"));
        if (d && !e || f) return b.focus(), !0
      }, this))
    },
    _getFullHeight: function() {
      return b.one(document).get("winHeight") - 2 * this.params.edgeMargin - this.controlsHeight - this.titleEl.get("offsetHeight")
    },
    dataReady: function() {
      var c;
      this.clearEdited();
      this._setState("EDITING");
      this.titleEl.removeClass("loading");
      this.params.initialData || (this.params.initialData = {});
      this.bodyHeight = this.params.height;
      this.controlsHeight = "standard" === this.params.style ? 65 : 60;
      if ("full" === this.params.verticalHeight || "fit" === this.params.verticalHeight) this.bodyHeight = this._getFullHeight();
      this.preferredHeight = this.bodyHeight;
      this.bodyEl.setStyle("width", this.params.width + "px");
      this.params.overlay && !this.params.doNotDisableBodyScroll && this.disableBodyScroll();
      if (this.moving) {
        this.moving = !1;
        this.updateTitle();
        if (this.definitionChanged) this.definitionChanged = !1, this.destroyBody(), this.destroyButtons(), this.render(), c = this._anim({
          node: this.bodyEl,
          duration: 0.25,
          easing: b.Easing.easeOutStrong,
          to: {
            height: this.bodyHeight
          }
        }), c.on("end", function() {
          this.fire("ready")
        }, this), c.run();
        else {
          if (this.rendered) this.setData(this.params.initialData), this.params.autoFocus && this.focusTab();
          else this.once("rendered", function() {
            this.setData(this.params.initialData);
            this.params.autoFocus && this.focusTab()
          }, this);
          this.fire("ready")
        }
        this.clearEdited()
      } else if (this.currentTabIndex = 0, this.currentTab = this.params.tabs[0], this.updateTitle(), this.params.loadingState) c = this._anim({
        node: this.el,
        to: {
          opacity: 1
        },
        duration: 0.15,
        easing: b.Easing.easeOutStrong
      }), c.run(), this.controlsEl && (c = this._anim({
        node: this.controlsEl,
        to: {
          height: this.controlsHeight
        },
        duration: 0.15,
        easing: b.Easing.easeOutStrong
      }), c.run()), c = this._anim({
        node: this.bodyEl,
        to: {
          height: this.bodyHeight
        },
        duration: 0.25,
        easing: b.Easing.easeOutStrong
      }), c.on("end", function() {
        this.render();
        this.fire("ready")
      }, this), c.run();
      else {
        this.render();
        var a = this,
          d = function() {
            a.el && a.el.setStyle("transform", null)
          };
        switch (this.params.showAnim) {
        case "custom":
          this.params.customShowAnim(this.el,

          function() {
            a.fire("ready")
          });
          break;
        case "slideDownUp":
          this.el.setStyles({
            marginTop: -this.el.height() + "px"
          });
          c = this.el.anim({
            marginTop: 0
          }, {
            duration: 0.3
          });
          c.on("end", function() {
            a.fire("ready")
          });
          c.run();
          break;
        case "fade":
          this.el.transition({
            opacity: {
              duration: 0.2,
              value: 1
            },
            easing: "ease-out"
          }, function() {
            a.fire("ready");
            d()
          });
          break;
        case "noshow":
          this.el.setStyles({
            display: "none"
          });
          a.fire("ready");
          break;
        default:
          JSTween.tween(this.el.getDOMNode(), {
            transform: {
              start: "scale(0.97)",
              stop: "scale(1)",
              time: 0,
              duration: 0.3,
              effect: "easeOut",
              onStop: b.bind(function() {
                a.fire("ready");
                d()
              }, this)
            },
            opacity: {
              start: 0,
              stop: 100,
              time: 0,
              duration: 0.3,
              effect: "easeOut"
            }
          })
        }
      }
      this.currentData = this.params.initialData;
      JSTween.play()
    },
    containsNode: function(b) {
      return this.bodyEl && this.bodyEl.contains(b)
    },
    activateTab: function(c, a) {
      this.hideErrors();
      var d = a ? a : b.DB.unpackData(c.target),
        e = parseInt(d.index, 10);
      this.currentTab || (this.currentTab = d);
      b.Array.each(this.params.tabs, function(a, b) {
        a.tabNavigationObj.toggleClass("active", b === e);
        b === e && this._showPanelByIndex(e)
      }, this)
    },
    _showField: function(b) {
      (this._isDialogField2(b) && b.get("visible") || !this._isDialogField2(b) && !b.config.hidden) && b.show(!0)
    },
    _showPanelByIndex: function(c) {
      if (c != this.currentTabIndex) {
        var a, d = b.Easing.easeBothStrong,
          e = this.params.tabs[c],
          f = this.params.tabs[this.currentTabIndex];
        this.params.noTabAnim || (e.tabPanelObj.animation && e.tabPanelObj.animation.stop(!0), f.tabPanelObj.animation && f.tabPanelObj.animation.stop(!0));
        a = f.tabFields;
        this.params.noTabAnim ? (f.tabPanelObj.setStyles({
          opacity: 1,
          zIndex: "300"
        }), f.tabPanelObj.addClass("hidden")) : (e.tabPanelObj.setStyles({
          left: (e.index > f.index ? 1 : -1) * this.params.width + "px",
          opacity: 1,
          zIndex: "300"
        }), f.tabPanelObj.setStyle("zIndex", "0"), a = this._anim({
          node: f.tabPanelObj,
          to: {
            left: (e.index < f.index ? 1 : -1) * this.params.width + "px"
          },
          duration: 0.5,
          easing: d
        }), a.on("end", function() {
          this.tab.addClass("hidden")
        }, {
          tab: f.tabPanelObj
        }), a.run(), f.tabPanelObj.animation = a);
        this.currentTab = e;
        this.currentTabIndex = c;
        this.updateTitle();
        e.tabPanelObj.removeClass("hidden");
        this.params.noTabAnim ? (a = this.currentTab.tabFields, b.Array.forEach(a, function(a) {
          this._showField(a)
        }, this), this.focusTab(), this.activateErrors()) : (a = this._anim({
          node: e.tabPanelObj,
          to: {
            left: 0
          },
          duration: 0.5,
          easing: d
        }), a.on("end", function() {
          b.Array.forEach(this.currentTab.tabFields, function(a) {
            this._showField(a)
          }, this);
          this.focusTab();
          this.activateErrors()
        }, this), a.run(), e.tabPanelObj.animation = a);
        this.currentTab.height && (this.currentTab.tabPanelObj.setStyle("height", f.height - 50), this._anim({
          node: this.bodyEl,
          to: {
            height: this.currentTab.height
          },
          duration: 0.5,
          easing: d
        }).run());
        this.bodyEl.toggleClass("scrollable", !! this.currentTab.scroll);
        this.currentTab.scroll ? this.bodyEl.plug(b.Squarespace.Plugin.ScrollLock) : this.bodyEl.unplug(b.Squarespace.Plugin.ScrollLock);
        this.fire("tab-shown", {
          name: this.currentTab.name,
          title: this.currentTab.title
        })
      }
    },
    render: function() {
      var c;
      this.rendered || (this.observedHeight = 0, this.tabs = [], this.tabsEl.setContent(""), b.Array.each(this.params.tabs, function(a, d) {
        c = b.DB.A("configuration-container-tab " + (0 === d ? "active" : ""), {
          href: "javascript:noop();",
          data: {
            index: d
          },
          html: this.params.tabs[d].tabTitle
        });
        1 < this.params.tabs.length && (this.params.tabs[d].noTabTitle && c.addClass("noTabTitle"), this.bodyEvents.push(c.on("click", function(a) {
          a.preventDefault();
          this.activateTab.apply(this, arguments)
        }, this)), this.tabsEl.append(c));
        this.params.tabs[d].tabNavigationObj = c;
        this.params.tabs[d].index = d;
        this.renderTab(this.params.tabs[d], 0 == d)
      }, this), this.renderButtons(), this.bodyHeight || (this.preferredHeight = this.bodyHeight = this.observedHeight + 14), "fit" == this.params.verticalHeight && this.bodyHeight > this.observedHeight && (this.bodyHeight = this.observedHeight), this.fire("rendered"), this.rendered = !0, (this.params.embedWithinEl && this.params.height || !this.params.embedWithinEl) && this.bodyEl.setStyle("height", this.bodyHeight + "px"), this.controlsEl && this.controlsEl.setStyle("height", this.controlsHeight + "px"), this._updatePosition(!0), this.params.autoFocus && this.focusTab())
    },
    getButtons: function() {
      return this.params.buttons
    },
    setButtons: function() {
      var c = arguments;
      1 === arguments.length && b.Lang.isArray(arguments[0]) && (c = arguments[0]);
      this.params.buttons = [];
      this.destroyButtons();
      c.forEach(function(a) {
        this.params.buttons.push(a)
      }, this);
      this.isVisible() && this.renderButtons()
    },
    _removeButton: function(c) {
      var a = b.Array.filter(this.getButtons(), function(a, b) {
        return a !== c && a.type !== c
      }, this);
      this.setButtons(a)
    },
    _getNextTabIndex: function() {
      return this.BUTTONS_BASE_IDX + this.lastTabIndex++
    },
    _disableButton: function(b) {
      b.addClass("disabled").one("input").set("disabled", "disabled")
    },
    renderButtons: function() {
      this.saveAndCloseButton = b.DB.INPUT("saveAndClose", {
        tabIndex: this._getNextTabIndex(),
        type: "button",
        value: "Save & Close"
      });
      this.saveButton = b.DB.INPUT("save", {
        tabIndex: this._getNextTabIndex(),
        type: "button",
        value: "Save"
      });
      this.cancelButton = b.DB.A("cancel", {
        tabIndex: this._getNextTabIndex(),
        href: "javascript:noop();",
        html: "Cancel"
      });
      this.removeButton = b.DB.INPUT("remove", {
        tabIndex: this._getNextTabIndex(),
        type: "button",
        value: "Remove"
      });
      this.buttonEvents.push(b.on("click",

      function(a) {
        a.halt();
        this._getButtonClickHandler("saveAndClose", this.saveAndClose)(a)
      }, this.saveAndCloseButton, this), b.on("click", this._getButtonClickHandler("save", this.save), this.saveButton), b.on("click", this._getButtonClickHandler("remove", this.remove), this.removeButton), b.on("click", this._getButtonClickHandler("close", this.close), this.cancelButton), b.on("click", this._getButtonClickHandler("cancel", this.cancelClick), this.cancelButton));
      var c, a = b.clone(this.params.buttons);
      i = this.params.buttons.length;
      for (0 < i && b.Squarespace.Utils.isDamaskEnabled() && a.reverse(); 0 <= --i;) {
        var d;
        if (c = a[i]) {
          switch (c.type) {
          case "cancel":
            this.cancelButton.set("innerHTML", c.title);
            d = b.Node.create('<div class="cancel-block"></div>');
            d.append(this.cancelButton);
            break;
          case "save":
            this.saveButton.set("value", c.title);
            d = b.Node.create('<div class="button-block"></div>');
            d.append(this.saveButton);
            break;
          case "remove":
            this.removeButton.set("value", c.title);
            d = b.Node.create('<div class="button-block"></div>');
            d.append(this.removeButton);
            break;
          case "saveAndClose":
            this.saveAndCloseButton.set("value", c.title);
            d = b.Node.create('<div class="button-block"></div>');
            d.append(this.saveAndCloseButton);
            break;
          default:
            var e;
            "text" === c.style ? (e = b.DB.A({
              href: "javascript:noop();",
              html: c.title
            }), d = b.Node.create('<div class="cancel-block"></div>')) : (e = b.DB.INPUT(c.className ? c.className : "", {
              type: "button",
              value: c.title
            }), d = b.Node.create('<div class="button-block"></div>'));
            d.append(e);
            this.publish("button-" + c.type, {
              emitFacade: !0,
              prefix: "EditingDialog",
              broadcast: 2
            });
            this.buttonEvents.push(b.on("click", this._getButtonClickHandler(c.type), e))
          }
          this.buttonHolder.append(d);
          c.disabled && this._disableButton(d)
        }
      }
    },
    cancelClick: function(b) {
      this.fire("cancel-click")
    },
    _getButtonClickHandler: function(c, a) {
      return b.bind(function(d) {
        this.fire("button-" + c) ? b.Lang.isFunction(a) && a.call(this, d) : d.halt();
        this.fire("button-click", {
          type: c
        })
      }, this)
    },
    _isOverlayAnimationRunning: function() {
      return b.Lang.isValue(this.saveOverlayAnimationManager) && this.saveOverlayAnimationManager.isRunning()
    },
    showSaveOverlay: function(c) {
      this.fire("show-save-overlay");
      if (!this.params.disableSaveOverlay) {
        this._isOverlayAnimationRunning() && this.saveOverlayAnimationManager.stop(!0);
        this.saveOverlay && this.saveOverlay.remove();
        var a = this.titleEl.get("offsetHeight");
        this.saveOverlay = b.Node.create('<div class="save-overlay">&nbsp;</div>');
        this.saveOverlay.setStyles({
          height: this.mainEl.get("offsetHeight") - a - 10 + "px",
          marginTop: a + 5 + "px",
          width: this.bodyEl.get("offsetWidth") + "px"
        });
        this.mainEl.append(this.saveOverlay);
        c && this.titleEl.addClass("save-overlay-active");
        this.saveOverlayAnimationManager = new b.Squarespace.AnimationManager;
        this.saveOverlayAnimationManager.push(this._anim({
          node: this.tabsEl,
          to: {
            opacity: 0
          },
          duration: 0.25,
          easing: b.Easing.easeOutStrong
        }));
        this.saveOverlayAnimationManager.push(this._anim({
          node: this.saveOverlay,
          to: {
            opacity: 0.8
          },
          duration: 0.25,
          easing: b.Easing.easeOutStrong
        }));
        this.autosaveEl && this.saveOverlayAnimationManager.push(this._anim({
          node: this.autosaveEl,
          to: {
            opacity: 0
          },
          duration: 0.25,
          easing: b.Easing.easeOutStrong
        }));
        "transparent" !== this.params.style && (this.saveOverlayAnimationManager.push(this._anim({
          node: this.titleEl,
          to: {
            opacity: 1
          },
          duration: 0.25,
          easing: b.Easing.easeOutStrong
        })), this.controlsEl && this.saveOverlayAnimationManager.push(this._anim({
          node: this.controlsEl.get("firstChild"),
          to: {
            opacity: 0
          },
          duration: 0.25,
          easing: b.Easing.easeOutStrong
        })));
        this.saveOverlayAnimationManager.on("end", function() {
          this.fire("save-overlay-shown")
        }, this);
        this.saveOverlayAnimationManager.run()
      }
    },
    hideSaveOverlay: function() {
      this.saveOverlay ? (this.fire("hide-save-overlay"), this.titleEl.removeClass("save-overlay-active"), this._isOverlayAnimationRunning() && this.saveOverlayAnimationManager.stop(!0), this.saveOverlayAnimationManager = new b.Squarespace.AnimationManager, this.saveOverlayAnimationManager.push(this._anim({
        node: this.saveOverlay,
        to: {
          opacity: 0
        },
        duration: 0.25,
        easing: b.Easing.easeOutStrong
      })), this.saveOverlayAnimationManager.push(this._anim({
        node: this.tabsEl,
        to: {
          opacity: 1
        },
        duration: 0.25,
        easing: b.Easing.easeOutStrong
      })), this.saveOverlayAnimationManager.on("end",

      function() {
        this.saveOverlay.remove();
        this.fire("save-overlay-hidden");
        this._setState("EDITING");
        this.updateTitle()
      }, this), this.saveOverlayAnimationManager.run()) : this._setState("EDITING")
    },
    allowEditing: function() {
      this.hideSaveOverlay();
      this.clearEdited();
      this.fire("editing-allowed");
      this._setState("EDITING")
    },
    save: function() {
      this._showLocalErrors() || (this._debug.log("save"), this.closeOnSend = !1, this._saveData())
    },
    _showLocalErrors: function() {
      this.hideErrors();
      var c = function(a) {
        return a.inActiveFrame
      },
      a = this.currentTab,
        d = function(b) {
          return b.inActiveFrame && b.tab.name === a.name
        }, e = b.Object.values(this.fields),
        e = this.params.validateActiveTabOnly ? e.filter(d, this) : e.filter(c, this),
        f = !1,
        g = {};
      b.Array.map(e, function(a) {
        return {
          field: a,
          errors: a.getErrors()
        }
      }).forEach(function(a) {
        var b = a.errors;
        b[b.length - 1] && (g[a.field.getName()] = b[b.length - 1], f = !0)
      }, this);
      f && this.showErrors(g);
      return f
    },
    saveAndShow: function() {
      this._showLocalErrors() || (this._debug.log("saveAndShow"), this.closeOnSend = !1, this._saveData(),
      this.show())
    },
    saveAndClose: function() {
      this._showLocalErrors() || (this.fire("preClose"), this._debug.log("saveAndClose"), this.params.closeable && (this.closeOnSend = !0), this._saveData())
    },
    _saveData: function() {
      this._debug.log("_saveData");
      this._isState("SAVING") ? this._debug.log("_saveData", "Exiting because dialog state is in SAVING") : (this.clearEdited(), this.hideErrors(), this._setState("SAVING"), this.updateTitle(), this.showSaveOverlay(), this._debug.log("_saveData", "fire", "send-requested"), this.fire("send-requested"))
    },
    remove: function(b) {
      b && b.halt();
      this.fire("remove-requested")
    },
    canClose: function(c, a) {
      if (!b.Array.every(this.childDialogs, function(a) {
        return a.canClose(this.saveAndClose, this.cancel)
      }, this)) return !1;
      if ((this.edited || this.editedSinceLastSave) && this.params.discardChangesConfirmation) {
        var d = !1,
          e;
        for (e in this.fields) this.fields[e].didDataChange() && (d = !0);
        if (d) return d = new b.Squarespace.Widgets.Confirmation({
          render: this.el.ancestor("body") || !0,
          style: b.Squarespace.Widgets.Confirmation.TYPE.CONFIRM_OR_REJECT,
          showOverlay: !0,
          "strings.confirm": "Save",
          "strings.reject": "Discard",
          "strings.title": "Review Changes",
          "strings.message": "You have made changes. Do you want to save or discard them?"
        }), d.on("confirm", function() {
          this.clearEdited();
          this._debug.log("canClose", "calling onSuccess()", c);
          c.call(this)
        }, this), d.on("reject", function() {
          this.clearEdited();
          this._debug.log("canClose", "calling onReject()", a);
          a.call(this)
        }, this), d.on("cancel", function() {
          b.Squarespace.EscManager.addTarget(this);
          this._debug.log("canClose", "onCancel");
          this.fire("cancel-close")
        }, this), this._debug.log("canClose", !1, "Showing confirmation dialog"), !1
      }
      this._debug.log("canClose", !0);
      return !0
    },
    close: function(b, a) {
      this._debug.log("close", b);
      if (!this._isState("EDITING") && !this._isState("SAVING")) this._debug.log("close", "Exiting because state is in Editing or Saving.");
      else {
        if (a) this._debug.log("canClose skipped due to force = true");
        else if (!1 === this.canClose(this.saveAndClose, this.cancel)) {
          this._debug.log("close", "Exiting because canClose came back false");
          return
        }
        this.fire("close");
        this.dismiss(!0)
      }
    },
    cancel: function(b) {
      this._debug.log("cancel", b);
      if (this._isState("EDITING") || this._isState("SAVING")) this.clearEdited(), this.fire("cancel"), this.dismiss(!1)
    },
    dismiss: function(c) {
      c = c ? b.bind(this.destroy, this) : b.bind(this._finishCancelation, this);
      this.fire("dismiss");
      this._setState("CLOSING");
      this.updateTitle();
      this.clearEdited();
      this._preDestroy();
      this.hideErrors();
      this.params.overlay && this.restoreBodyScroll();
      switch (this.params.hideAnim) {
      case "noHideAnimation":
        c();
        break;
      case "custom":
        this.params.customHideAnim(this.el, c);
        break;
      case "slideDownUp":
        var a = this._anim({
          node: this.el,
          to: {
            marginTop: -this.el.height() + "px"
          },
          duration: 0.3,
          easing: b.Easing.easeOut
        });
        a.on("end", c);
        a.run();
        break;
      default:
        JSTween.tween(this.el.getDOMNode(), {
          transform: {
            start: "scale(1)",
            stop: "scale(0.97)",
            time: 0,
            duration: 0.4,
            effect: "easeOut",
            onStop: c
          },
          opacity: {
            start: 100,
            stop: 0,
            time: 0,
            duration: 0.4,
            effect: "easeOut"
          }
        })
      }
      this.params.overlay && this.overlayEl && (this.overlayEl.destroying = !0, JSTween.tween(this.overlayEl.getDOMNode(), {
        opacity: {
          start: 100 * this.params.overlay,
          stop: 0,
          time: 0,
          duration: 0.4,
          effect: "easeOut",
          onStop: b.bind(function() {
            this.overlayEl && (this.overlayEl.remove(), this.overlayEl = null)
          }, this)
        }
      }));
      JSTween.play()
    },
    _finishCancelation: function() {
      this.fire("render-anchor", this.getInitialData());
      this.destroy();
      this._setState("CLOSED");
      this.fire("canceled")
    },
    _preDestroy: function() {
      var c = 0,
        a;
      this._preDestroyCalled = !0;
      c = b.Squarespace.OPEN_DIALOGS.indexOf(this); - 1 !== c && b.Squarespace.OPEN_DIALOGS.splice(c, 1);
      0 === b.Squarespace.OPEN_DIALOGS.length && b.one(document.body).removeClass("dialog-open");
      this._applyMethodToChildren("close");
      c = 0;
      for (a = this.timers.length; c < a; ++c) this.timers[c].cancel();
      this.timers = [];
      b.Squarespace.EscManager && b.Squarespace.EscManager.removeTarget(this);
      this.activeFlyout && this.activeFlyout.field.closeFlyout();
      b.Squarespace.ToolTipManager && this.params.disableTips && b.Squarespace.ToolTipManager.enableTooltips();
      this.firstShowEvent && (this.firstShowEvent.detach(), this.firstShowEvent = null);
      this.anchorEl && this.anchorEl._node && this.anchorEl.removeClass("targeted")
    },
    getErrors: function() {
      var b = 0,
        a = {}, d;
      for (d in this.fields) {
        var e = this.fields[d],
          f = !1;
        if (f = this._isDialogField2(e) ? e.get("required") && e.isEmpty() : e.config.required && ("" === e.getValue() || 0 === e.getValue())) a[e.getName()] = "This is a required field.", b++
      }
      return {
        errors: b,
        errorSet: a
      }
    },
    send: function() {
      var b = this.getErrors(),
        a = b.errorSet;
      b.errors ? (this.showErrors(a), this.fire("local-errors")) : (this.closeOnSend && (this.saved = !0, this.closeOnSend = !1, this.close()), this.fire("sent"))
    },
    updateAutoSave: function(c, a) {
      if (this.autosaveEl.inDoc()) {
        this.autosaveEl.setStyle("display", "block");
        var d = this._anim({
          node: this.autosaveEl,
          from: {
            opacity: 1
          },
          to: {
            opacity: 0.5
          },
          duration: 0.25,
          easing: b.Easing.easeOutStrong
        });
        d.on("end", function() {
          c ? this.autosaveEl.set("innerHTML", c) : (this.autosaveEl.set("innerHTML", 'Last saved <span class="time"></span>.'), this.autosaveEl.one(".time").plug(b.Squarespace.RelativeTimeDisplay));
          a ? this.autosaveEl.addClass("error") : this.autosaveEl.removeClass("error");
          this.autosaveEl.ancestor("body") && this._anim({
            node: this.autosaveEl,
            from: {
              opacity: 0.5
            },
            to: {
              opacity: 1
            },
            duration: 0.25,
            easing: b.Easing.easeOutStrong
          }).run()
        }, this);
        d.run()
      }
      this.editedSinceLastSave = !1
    },
    _recordFieldData: function(c, a) {
      b.Object.each(a.fields, function(a) {
        if (b.Lang.isFunction(a.getValue)) {
          var e = a.getValue();
          c[a.getName()] = e;
          if (void 0 !== a.getAssociatedVars) for (var e = a.getAssociatedVars(), f = 0; f < e.length; f++) c[e[f].name] = e[f].value
        }
        a.fields && this._recordFieldData(c, a)
      }, this)
    },
    getData: function() {
      var b = {}, a;
      for (a in this.params.initialData) b[a] = this.params.initialData[a];
      this._recordFieldData(b, this);
      return b
    },
    getField: function(c) {
      var a = this.fields[c];
      if (b.Lang.isValue(a)) return a;
      for (var d in this.fields) if (this.fields[d].getField && (a = this.fields[d].getField(c), b.Lang.isValue(a))) return a
    },
    getSection: function(b) {
      return this.sections[b]
    },
    _renderFields: function(c, a, d, e) {
      b.Array.each(a, function(f, g) {
        var h;
        if (f) {
          var k = f.type;
          !e && !this.params.dontSetWidthOnFields && (e = this.params.width - 60);
          e && !isNaN(e) && (this._debug.log("availableWidth: ", e),
          this._debug.log("fieldConfig.width: ", f.width), h = (f.ctor && f.config ? f.config.width : f.width) || 1, h = 1 >= h ? Math.floor(h * e) : h);
          "splitter" === k ? this._renderSplitter(f, g, a, c, d, e, h) : "multi-frame" === k ? this._renderMultiFrame(f, g, a, c, d, e, h) : "section" === k ? this._renderSection(f, g, a, c, d, e, h) : "stack" === k ? this._renderStack(f, g, a, c, d, e, h) : f.ctor && f.ctor === b.Squarespace.DialogFields.MultiFrame ? this._renderDF2MultiFrame(f, g, a, c, d, e, h) : this._renderField(f, g, a, c, d, e, h)
        }
      }, this)
    },
    _renderSplitter: function(c, a, d, e, f, g, h) {
      a = b.Node.create('<div class="split-field clear ' + (1 < c.fields.length ? "padding-adjust" : "") + '"></div>');
      c.width || (c.width = 1);
      g && !isNaN(g) ? (a.setStyle("width", Math.round(c.width * g) + 30 + "px"), g -= 30 * (c.fields.length - 1)) : g = f.get("offsetWidth") - 30 * (c.fields.length - 1) - 60;
      f.append(a);
      this._renderFields(e, c.fields, a, g);
      c.countHeight && (this._takenHeight += a.get("offsetHeight"));
      c.hidden && a.hide()
    },
    _renderMultiFrame: function(c, a, d, e, f, g, h) {
      a = new b.Squarespace.DialogFieldGenerators[c.type](c, this.params.initialData,
      this);
      a.type = c.type;
      a.append(e, d, f, g);
      a.getName() ? this.fields[a.getName()] = a : this._noNameFields.push(a)
    },
    _renderDF2MultiFrame: function(c, a, d, e, f, g, h) {
      a = b.merge(c, c.config, {
        dialog: this,
        data: this.params.initialData[c.config.name],
        render: f
      });
      c = new c.ctor(a);
      g && c.get("boundingBox").setStyle("width", h + "px");
      (g = c.getName() || c.get("name")) ? this.fields[g] = c : this._noNameFields.push(c);
      c.each(function(a) {
        var b = a.getName() || a.get("name");
        b ? this.fields[b] = a : this._noNameFields.push(a);
        e.tabFields.push(a);
        a.tab = e
      }, this)
    },
    _renderSection: function(c, a, d, e, f, g, h) {
      a = b.Node.create('<div class="section-inner clear"></div>');
      d = "section-field container-field-wrapper field-wrapper clear ";
      void 0 !== c.style && (d += c.style);
      d = b.Node.create('<div class="' + d + '"></div>');
      d.append(a);
      f.append(d);
      c.width || (c.width = 1);
      (f = Math.round(c.width * g)) && d.setStyle("width", f + "px");
      g = Math.round(c.width * g);
      this._renderFields(e, c.fields, a, g);
      c.hidden && d.hide();
      c.name && (this.sections[c.name] = d)
    },
    _renderStack: function(c, a, d, e, f, g, h) {
      (h = g) || (h = this.params.width - 60);
      g = b.Node.create('<div class="stack-field stack-field-wrapper clear"></div>');
      f.append(g);
      c.width || (c.width = 1);
      c["float"] && g.setStyle("float", c["float"]);
      g.setStyle("width", Math.round(c.width * h) + "px");
      a !== d.length - 1 && g.setStyle("paddingRight", "30px");
      a = Math.round(c.width * h);
      this._renderFields(e, c.fields, g, a);
      c.hidden && g.hide();
      c.name && (this.sections[c.name] = g)
    },
    _renderField: function(c, a, d, e, f, g, h) {
      var k;
      a = b.merge(c, c.config || {}, {
        dialog: this,
        render: f
      });
      d = a.name || c.name;
      this._debug.isTimingEnabled() && this._debug.time("render field: " + d);
      b.Lang.isUndefined(a.data) && (this.params.initialData && !b.Lang.isUndefined(d)) && (a.data = this.params.initialData[d], a.panel = this);
      if (c.ctor) k = new c.ctor(a);
      else {
        if (!c.type) throw console.error("dialog: field type was", c.type, ", and constructor was", c.ctor), console.error("dialog: config was", c), "Could not find field constructor or field type.";
        var l = this._convertToDialogField2Name(c.type),
          l = b.namespace("Squarespace.DialogFields")[l];
        if (!l && !b.Squarespace.DialogFieldGenerators[c.type]) throw "Unknown field type: " + c.type;
        this._debug.log("Generating field type: ", c.type);
        l ? k = new l(a) : (k = new b.Squarespace.DialogFieldGenerators[c.type](c, this.params.initialData, this), k.type = c.type, k.append(f))
      }
      c.verticalSpan && (this._verticalEl = k);
      c.countHeight && !c.verticalSpan && (k.getTakenHeight ? this._takenHeight += k.getTakenHeight() : (this._takenHeight += k.get("boundingBox").get("offsetHeight"), this._takenHeight += parseInt(k.get("boundingBox").getStyle("marginTop"), 10), this._takenHeight += parseInt(k.get("boundingBox").getStyle("marginBottom"),
      10)));
      g && (this._isDialogField2(k) ? (this._debug.log("setting width", h), k.get("boundingBox").setStyle("width", h + "px")) : k.html && (this._debug.log("setting width", h), k.html.setStyle("width", h + "px")));
      k.resize && k.resize();
      c.hidden && k.temporaryHide(!0);
      this._isDialogField2(k) ? this.bodyEvents.push(k.on("dataStateChange", function(a) {
        a = a.newVal;
        k.getProperty("DATA_STATES");
        this.fire("field-loading-change", {
          field: k,
          loading: a === a.LOADING
        })
      }, this)) : this.bodyEvents.push(k.on("loadingChange", function(a) {
        this.fire("field-loading-change", {
          field: k,
          loading: a.newVal
        })
      }, this));
      k.getName() ? this.fields[k.getName()] = k : this._noNameFields.push(k);
      e.tabFields.push(k);
      k.tab = e;
      this._debug.isTimingEnabled() && this._debug.timeEnd("render field: " + d)
    },
    _isDialogField2: function(c) {
      return b.Lang.isValue(c) && b.Lang.isValue(b.Squarespace.DialogField2) && c instanceof b.Squarespace.DialogField2
    },
    _setState: function(c) {
      b.Lang.isValue(b.Squarespace.DialogStates[c]) ? this.state = b.Squarespace.DialogStates[c] : console.warn("[Dialog] Invalid state.")
    },
    _isState: function(c) {
      return b.Lang.isString(c) ? this.state === b.Squarespace.DialogStates[c] : this.state === c
    },
    resizeVerticalFields: function() {
      b.Array.each(this.verticalFields, function(b, a) {
        var d = this.getBodyHeight() - b._takenHeight;
        this._isDialogField2(b) ? (d -= parseInt(b.get("boundingBox").getStyle("marginTop"), 10), d -= parseInt(b.get("boundingBox").getStyle("marginBottom"), 10), b.set("height", d)) : b.setHeight(d)
      }, this)
    },
    renderTab: function(c, a) {
      var d = b.Node.create('<div class="tab-wrapper"></div>');
      d.setStyles({
        width: this.params.width + "px",
        left: a ? "0px" : this.params.width + "px"
      });
      this.bodyEl.append(d);
      c.tabFields = [];
      this._verticalEl = null;
      this._takenHeight = 0;
      this._renderFields(c, c.fields || [], d);
      c.noTabTitle && d.addClass("noTabTitle");
      if (this._verticalEl) {
        var e = parseInt(d.getStyle("paddingTop"), 10);
        isNaN(parseInt(d.getStyle("marginTop"), 10)) || (e += parseInt(d.getStyle("marginTop"), 10));
        this._takenHeight += e;
        var f = this._isDialogField2(this._verticalEl);
        !f && !this._verticalEl.setHeight && console.error("No setHeight for vertical el: ", this._verticalEl);
        e = this.getBodyHeight() - this._takenHeight;
        f ? (f = this._verticalEl.get("boundingBox"), e -= parseInt(f.getStyle("marginTop"), 10), e -= parseInt(f.getStyle("marginBottom"), 10), this._verticalEl.set("height", e)) : this._verticalEl.setHeight(e);
        this._verticalEl._takenHeight = this._takenHeight;
        this.verticalFields.push(this._verticalEl)
      }
      c.tabPanelObj = d;
      this.observedHeight += d.get("offsetHeight");
      a && c.tabFields && b.Array.forEach(c.tabFields, function(a) {
        this._showField(a)
      }, this);
      this.bodyEl.toggleClass("scrollable", !! c.scroll);
      a || d.addClass("hidden");
      return d
    },
    _convertToDialogField2Name: function(c) {
      c = b.Squarespace.Utils.slugify(c).split("-");
      for (var a = c.length - 1; 0 <= a; a--) c[a].capitalize && (c[a] = c[a].capitalize());
      return c.join("") + "Field"
    },
    _destroyFields: function() {
      this.fields && (b.Object.each(this.fields, function(b, a) {
        b.destroy()
      }, this), this._destroyNoNameFields());
      this.fields = {}
    },
    _destroyNoNameFields: function() {
      b.Array.each(this._noNameFields, function(b) {
        b && b.destroy()
      }, this);
      this._noNameFields = []
    },
    destroyBody: function() {
      this.rendered = !1;
      this._destroyFields();
      this._isOverlayAnimationRunning() && this.saveOverlayAnimationManager.destroy(!0);
      this.bodyEl && this.bodyEl._node && this.bodyEl.empty();
      b.Array.each(this.params.tabs, function(b) {
        b.tabPanelObj = null;
        b.tabNavigationObj = null;
        b.tabFields = null
      });
      this._detachEventArray(this.bodyEvents);
      this.bodyEvents = []
    },
    _detachEventArray: function(c) {
      b.Array.each(c, function(a) {
        a.detach()
      })
    },
    destroyButtons: function() {
      this._detachEventArray(this.buttonEvents);
      this.buttonEvents = [];
      this.buttonHolder && this.buttonHolder._node && this.buttonHolder.set("innerHTML", "")
    },
    _destroy: function() {
      this.preDestroyCalled || this._preDestroy();
      this.bodyHeight = this.currentErrors = this.destroyTimer = null;
      this.destroyBody();
      this.destroyButtons();
      this.overlayEl && !this.overlayEl.destroying && (this.overlayEl.remove(), this.overlayEl = null);
      this._onBeforeUnloadEvt && this._onBeforeUnloadEvt.detach();
      this._detachEventArray(this.globalEvents);
      this.dd && (this.dd.destroy(), this.dd = null);
      this.globalEvents = [];
      this.el && (this.el.remove(!0), this.bodyEl = this.el = null);
      this.params.parentDialog && (this.params.parentDialog.removeChildDialog(this),
      this.params.parentDialog = null);
      this._setState("CLOSED");
      this.anchorEl = this.position = null
    }
  });
  b.augment(b.Squarespace.EditingDialog, b.EventTarget)
}, "1.0", {
  requires: "anim attribute datatype-date dd json node node-event-simulate node-focusmanager squarespace-animation-manager squarespace-beforeunload squarespace-debugger squarespace-dialog-field-2 squarespace-dialog-fields squarespace-dialog-fields-generators squarespace-dombuilder squarespace-escmanager squarespace-gizmo squarespace-plugin-scroll-lock squarespace-ui-base squarespace-widgets-confirmation thirdparty-jstween transition".split(" ")
});
YUI.add("squarespace-node-flyout", function(b) {
  b.namespace("Squarespace.Animations").Flyout = b.Base.create("flyoutPlugin", b.Plugin.Base, [], {
    initializer: function(c) {
      this._mask = b.Node.create('<div class="flyout-animation-wrapper sqs-flyout-mask"></div>');
      this._mask.setStyles({
        position: "fixed",
        overflow: this.get("overflow")
      });
      this._mask.setStyle("z-index", "200000");
      this._mask.setStyle("zIndex", "200000");
      this._isHiding = this._isShowing = !1
    },
    destructor: function() {
      this._anim && this._anim.stop().destroy();
      this._mask.remove(!0);
      b.detachAll(this.get("id") + "|*")
    },
    _onScroll: function(b) {
      var a = this.get("host");
      (b.target.contains(a) || b.currentTarget.contains(a)) && this._updateMaskPosition()
    },
    _onResize: function() {
      this._updateMaskPosition()
    },
    show: function() {
      !this.get("visible") && !this._isShowing && this._animateFlyout(!0)
    },
    hide: function() {
      this.get("visible") && !this._isHiding && this._animateFlyout(!1)
    },
    _measureNode: function(c) {
      return b.Squarespace.Utils.measureNode(c)
    },
    _animateFlyout: function(c) {
      var a = this._mask,
        d = this.get("node");
      d.get("region");
      var e = this._measureNode(d),
        f = this._getInitialFlyoutOffset();
      this._updateMaskPosition();
      this._anim && (this._anim.stop(!0), this._anim = null);
      c && (a.setStyles({
        height: e.height,
        width: e.width + 1
      }), d.setStyles({
        position: "absolute",
        top: f.yOffset,
        left: f.xOffset
      }), a.appendChild(d), (this.get("renderTarget") || b.one("body")).appendChild(a));
      this._anim = new b.Anim({
        duration: this.get("duration"),
        easing: this.get("easing"),
        node: d,
        to: {
          top: c ? 0 : f.yOffset,
          left: c ? 0 : f.xOffset
        }
      });
      this.get("animateOpacity") && (this._anim.set("from.opacity",
      c ? 0 : 1), this._anim.set("to.opacity", c ? 1 : 0));
      this._anim.on(this._yuid + "|end", function() {
        this._anim = null;
        c ? (this._isShowing = !1, this._mousewheelEvent = b.on(this.get("id") + "|mousewheel", this._onScroll, this), this._resizeEvent = b.one(window).on(this.get("id") + "|resize", this._onResize, this)) : (this._isHiding = !1, d.remove(), a.remove(), this._mousewheelEvent && (this._mousewheelEvent.detach(), this._mousewheelEvent = null), this._resizeEvent && (this._resizeEvent.detach(), this._resizeEvent = null));
        this.set("visible", c);
        this.fire(c ? "shown" : "hidden", {
          flyout: d
        });
        this.get("host").fire("flyout-" + c ? "shown" : "hidden", {
          flyout: d
        })
      }, this);
      c ? this._isShowing = !0 : this._isHiding = !0;
      d.inDoc() && this._anim.run()
    },
    _updateMaskPosition: function() {
      if (this._mask) {
        var b = this._mask,
          a = this._getIntendedMaskPosition();
        b.setStyles({
          left: a.x,
          top: a.y
        });
        return a
      }
    },
    _getInitialFlyoutOffset: function() {
      var c = b.Squarespace.Animations.Flyout,
        a = this.get("alignment"),
        d = this._measureNode(this.get("node")),
        e;
      switch (a) {
      case c.LT:
      case c.LC:
      case c.LR:
        e = d.width;
        break;
      case c.TL:
      case c.TC:
      case c.TR:
      case c.BL:
      case c.BC:
      case c.BC:
        e = 0;
        break;
      case c.RT:
      case c.RC:
      case c.RB:
        e = -1 * d.width;
        break;
      default:
        throw "Flyout: This should never happened, check your alignment settings";
      }
      switch (a) {
      case c.LT:
      case c.LC:
      case c.LB:
      case c.RT:
      case c.RC:
      case c.RB:
        c = 0;
        break;
      case c.BL:
      case c.BC:
      case c.BR:
        c = -1 * d.height;
        break;
      case c.TL:
      case c.TC:
      case c.TR:
        c = d.height;
        break;
      default:
        throw "Flyout: This should never happened, check your alignment settings";
      }
      return {
        xOffset: e,
        yOffset: c
      }
    },
    _getIntendedMaskPosition: function() {
      var c = b.Squarespace.Animations.Flyout,
        a = this.get("alignment"),
        d = this.get("host").get("region"),
        e = this._measureNode(this.get("node")),
        f;
      f = b.DOM.docScrollY();
      var g = b.DOM.docScrollX();
      d.top -= f;
      d.bottom -= f;
      d.left -= g;
      d.right -= g;
      switch (a) {
      case c.RT:
      case c.RC:
      case c.RB:
        f = d.right;
        break;
      case c.LT:
      case c.LC:
      case c.LB:
        f = d.left - e.width;
        break;
      case c.TL:
      case c.BL:
        f = d.left;
        break;
      case c.TC:
      case c.BC:
        f = d.left + d.width / 2 - e.width / 2;
        break;
      case c.TR:
      case c.BR:
        f = d.right - e.width;
        break;
      default:
        throw "Flyout: This should never happened, check your alignment settings";
      }
      switch (a) {
      case c.TL:
      case c.TC:
      case c.TR:
        c = d.top - e.height;
        break;
      case c.LT:
      case c.RT:
        c = d.top;
        break;
      case c.LC:
      case c.RC:
        c = d.top + d.height / 2 - e.height / 2;
        break;
      case c.LB:
      case c.RB:
        c = d.bottom - e.height;
        break;
      case c.BL:
      case c.BC:
      case c.BR:
        c = d.bottom;
        break;
      default:
        throw "Flyout: This should never happened, check your alignment settings";
      }
      return {
        x: f,
        y: c
      }
    }
  }, {
    NS: "flyoutPlugin",
    TL: "tl",
    TC: "tc",
    TR: "tr",
    RT: "rt",
    RC: "rc",
    RB: "rb",
    BC: "bc",
    BL: "bl",
    BR: "br",
    LT: "lt",
    LC: "lc",
    Lb: "lb",
    ATTRS: {
      duration: {
        value: 0.3,
        validator: b.Lang.isNumber
      },
      easing: {
        value: b.Easing.easeOutStrong
      },
      alignment: {
        value: "rt",
        validator: function(c) {
          var a = b.Squarespace.Animations.Flyout;
          switch (c) {
          case a.TL:
          case a.TC:
          case a.TR:
          case a.LT:
          case a.RT:
          case a.LC:
          case a.RC:
          case a.LB:
          case a.RB:
          case a.BL:
          case a.BC:
          case a.BR:
            return !0;
          default:
            return console.warn(this.name + ": Invalid alignment value (" + c + ")"), !1
          }
        }
      },
      node: {
        value: null
      },
      animateOpacity: {
        value: !0
      },
      renderTarget: {
        valueFn: function() {
          var c = this.get("host");
          return c instanceof b.Node && c.ancestor("body") ? c.ancestor("body") : b.one("body")
        }
      },
      overflow: {
        value: "hidden"
      },
      visible: {
        value: !1
      }
    }
  })
}, "1.0", {
  requires: ["plugin", "node", "squarespace-util"]
});
YUI.add("node-focusmanager", function(b, c) {
  var a = {
    37: !0,
    38: !0,
    39: !0,
    40: !0
  }, d = {
    a: !0,
    button: !0,
    input: !0,
    object: !0
  }, e = b.Lang,
    f = b.UA,
    g = function() {
      g.superclass.constructor.apply(this, arguments)
    };
  g.ATTRS = {
    focused: {
      value: !1,
      readOnly: !0
    },
    descendants: {
      getter: function(a) {
        return this.get("host").all(a)
      }
    },
    activeDescendant: {
      setter: function(a) {
        var c = e.isNumber,
          d = b.Attribute.INVALID_VALUE,
          f = this._descendantsMap,
          g = this._descendants,
          p;
        c(a) ? a = p = a : a instanceof b.Node && f ? (p = f[a.get("id")], a = c(p) ? p : d) : a = d;
        g && (g = g.item(p)) && g.get("disabled") && (a = d);
        return a
      }
    },
    keys: {
      value: {
        next: null,
        previous: null
      }
    },
    focusClass: {},
    circular: {
      value: !0
    }
  };
  b.extend(g, b.Plugin.Base, {
    _stopped: !0,
    _descendants: null,
    _descendantsMap: null,
    _focusedNode: null,
    _lastNodeIndex: 0,
    _eventHandlers: null,
    _initDescendants: function() {
      var a = this.get("descendants"),
        c = {}, d = -1,
        f, g = this.get("activeDescendant"),
        p, s, r = 0;
      e.isUndefined(g) && (g = -1);
      if (a) {
        f = a.size();
        for (r = 0; r < f; r++) p = a.item(r), - 1 === d && !p.get("disabled") && (d = r), 0 > g && 0 === parseInt(p.getAttribute("tabIndex", 2),
        10) && (g = r), p && p.set("tabIndex", - 1), s = p.get("id"), s || (s = b.guid(), p.set("id", s)), c[s] = r;
        0 > g && (g = 0);
        p = a.item(g);
        if (!p || p.get("disabled")) p = a.item(d), g = d;
        this._lastNodeIndex = f - 1;
        this._descendants = a;
        this._descendantsMap = c;
        this.set("activeDescendant", g);
        p && p.set("tabIndex", 0)
      }
    },
    _isDescendant: function(a) {
      return a.get("id") in this._descendantsMap
    },
    _removeFocusClass: function() {
      var a = this._focusedNode,
        b = this.get("focusClass"),
        c;
      b && (c = e.isString(b) ? b : b.className);
      a && c && a.removeClass(c)
    },
    _detachKeyHandler: function() {
      var a = this._prevKeyHandler,
        b = this._nextKeyHandler;
      a && a.detach();
      b && b.detach()
    },
    _preventScroll: function(b) {
      a[b.keyCode] && this._isDescendant(b.target) && b.preventDefault()
    },
    _fireClick: function(a) {
      var b = a.target,
        c = b.get("nodeName").toLowerCase();
      13 === a.keyCode && (!d[c] || "a" === c && !b.getAttribute("href")) && b.simulate("click")
    },
    _attachKeyHandler: function() {
      this._detachKeyHandler();
      var a = this.get("keys.next"),
        c = this.get("keys.previous"),
        d = this.get("host"),
        e = this._eventHandlers;
      c && (this._prevKeyHandler = b.on("key",
      b.bind(this._focusPrevious, this), d, c));
      a && (this._nextKeyHandler = b.on("key", b.bind(this._focusNext, this), d, a));
      f.opera && e.push(d.on("keypress", this._preventScroll, this));
      f.opera || e.push(d.on("keypress", this._fireClick, this))
    },
    _detachEventHandlers: function() {
      this._detachKeyHandler();
      var a = this._eventHandlers;
      a && (b.Array.each(a, function(a) {
        a.detach()
      }), this._eventHandlers = null)
    },
    _attachEventHandlers: function() {
      var a = this._descendants,
        c, d;
      a && a.size() && (a = this._eventHandlers || [], c = this.get("host").get("ownerDocument"),
      0 === a.length && (a.push(c.on("focus", this._onDocFocus, this)), a.push(c.on("mousedown", this._onDocMouseDown, this)), a.push(this.after("keysChange", this._attachKeyHandler)), a.push(this.after("descendantsChange", this._initDescendants)), a.push(this.after("activeDescendantChange", this._afterActiveDescendantChange)), d = this.after("focusedChange", b.bind(function(a) {
        a.newVal && (this._attachKeyHandler(), d.detach())
      }, this)), a.push(d)), this._eventHandlers = a)
    },
    _onDocMouseDown: function(a) {
      var b = this.get("host"),
        c = a.target,
        d = b.contains(c),
        e, g = function(a) {
          var c = !1;
          a.compareTo(b) || (c = this._isDescendant(a) ? a : g.call(this, a.get("parentNode")));
          return c
        };
      d && ((e = g.call(this, c)) ? c = e : !e && this.get("focused") && (this._set("focused", !1), this._onDocFocus(a)));
      if (d && this._isDescendant(c)) this.focus(c);
      else if (f.webkit && this.get("focused") && (!d || d && !this._isDescendant(c))) this._set("focused", !1), this._onDocFocus(a)
    },
    _onDocFocus: function(a) {
      a = this._focusTarget || a.target;
      var b = this.get("focused"),
        c = this.get("focusClass"),
        d = this._focusedNode,
        e;
      this._focusTarget && (this._focusTarget = null);
      this.get("host").contains(a) ? (e = this._isDescendant(a), !b && e ? b = !0 : b && !e && (b = !1)) : b = !1;
      c && (d && (!d.compareTo(a) || !b) && this._removeFocusClass(), e && b && (c.fn ? (a = c.fn(a), a.addClass(c.className)) : a.addClass(c), this._focusedNode = a));
      this._set("focused", b)
    },
    _focusNext: function(a, b) {
      var c = b || this.get("activeDescendant"),
        d;
      this._isDescendant(a.target) && c <= this._lastNodeIndex && (c += 1, c === this._lastNodeIndex + 1 && this.get("circular") && (c = 0), (d = this._descendants.item(c)) && (d.get("disabled") ? this._focusNext(a, c) : this.focus(c)));
      this._preventScroll(a)
    },
    _focusPrevious: function(a, b) {
      var c = b || this.get("activeDescendant"),
        d;
      this._isDescendant(a.target) && 0 <= c && (c -= 1, - 1 === c && this.get("circular") && (c = this._lastNodeIndex), (d = this._descendants.item(c)) && (d.get("disabled") ? this._focusPrevious(a, c) : this.focus(c)));
      this._preventScroll(a)
    },
    _afterActiveDescendantChange: function(a) {
      var b = this._descendants.item(a.prevVal);
      b && b.set("tabIndex", - 1);
      (b = this._descendants.item(a.newVal)) && b.set("tabIndex", 0)
    },
    initializer: function(a) {
      this.start()
    },
    destructor: function() {
      this.stop();
      this.get("host").focusManager = null
    },
    focus: function(a) {
      e.isUndefined(a) && (a = this.get("activeDescendant"));
      this.set("activeDescendant", a, {
        src: "UI"
      });
      if (a = this._descendants.item(this.get("activeDescendant"))) a.focus(), f.opera && "button" === a.get("nodeName").toLowerCase() && (this._focusTarget = a)
    },
    blur: function() {
      var a;
      if (this.get("focused")) {
        if (a = this._descendants.item(this.get("activeDescendant"))) a.blur(), this._removeFocusClass();
        this._set("focused", !1, {
          src: "UI"
        })
      }
    },
    start: function() {
      this._stopped && (this._initDescendants(), this._attachEventHandlers(), this._stopped = !1)
    },
    stop: function() {
      this._stopped || (this._detachEventHandlers(), this._focusedNode = this._descendants = null, this._lastNodeIndex = 0, this._stopped = !0)
    },
    refresh: function() {
      this._initDescendants();
      this._eventHandlers || this._attachEventHandlers()
    }
  });
  g.NAME = "nodeFocusManager";
  g.NS = "focusManager";
  b.namespace("Plugin");
  b.Plugin.NodeFocusManager = g
}, "3.17.2", {
  requires: "attribute node plugin node-event-simulate event-key event-focus".split(" ")
});
YUI.add("view", function(b, c) {
  function a() {
    a.superclass.constructor.apply(this, arguments)
  }
  b.View = b.extend(a, b.Base, {
    containerTemplate: "<div/>",
    events: {},
    template: "",
    _allowAdHocAttrs: !0,
    initializer: function(a) {
      a || (a = {});
      a.containerTemplate && (this.containerTemplate = a.containerTemplate);
      a.template && (this.template = a.template);
      this.events = a.events ? b.merge(this.events, a.events) : this.events;
      this.after("containerChange", this._afterContainerChange)
    },
    destroy: function(b) {
      if (b && (b.remove || b["delete"])) this.onceAfter("destroy",

      function() {
        this._destroyContainer()
      });
      return a.superclass.destroy.call(this)
    },
    destructor: function() {
      this.detachEvents();
      delete this._container
    },
    attachEvents: function(a) {
      var c = this.get("container"),
        f = b.Object.owns,
        g, h, k, l;
      this.detachEvents();
      a || (a = this.events);
      for (l in a) if (f(a, l)) for (k in h = a[l], h) f(h, k) && (g = h[k], "string" === typeof g && (g = this[g]), g && this._attachedViewEvents.push(c.delegate(k, g, l, this)));
      return this
    },
    create: function(a) {
      return a ? b.one(a) : b.Node.create(this.containerTemplate)
    },
    detachEvents: function() {
      b.Array.each(this._attachedViewEvents,

      function(a) {
        a && a.detach()
      });
      this._attachedViewEvents = [];
      return this
    },
    remove: function() {
      var a = this.get("container");
      a && a.remove();
      return this
    },
    render: function() {
      return this
    },
    _destroyContainer: function() {
      var a = this.get("container");
      a && a.remove(!0)
    },
    _getContainer: function(a) {
      this._container || (a ? (this._container = a, this.attachEvents()) : (a = this._container = this.create(), this._set("container", a)));
      return a
    },
    _afterContainerChange: function() {
      this.attachEvents(this.events)
    }
  }, {
    NAME: "view",
    ATTRS: {
      container: {
        getter: "_getContainer",
        setter: b.one,
        writeOnce: !0
      }
    },
    _NON_ATTRS_CFG: ["containerTemplate", "events", "template"]
  })
}, "3.17.2", {
  requires: ["base-build", "node-event-delegate"]
});
YUI.add("squarespace-dialog-field-legacy-interface", function(b) {
  var c = b.namespace("Squarespace").DialogFieldLegacyInterface = function(a) {
    this.inActiveFrame = !0;
    b.Lang.isFunction(this.hideError) && b.Do.after(function() {
      var b = a.dialog;
      b && b.clearError(this)
    }, this, "hideError", this);
    b.Do.after(function() {
      this.after(this.get("id") + "|dataChange", function(b) {
        var c = a.dialog;
        c && !b.silent && c.fire("datachange", this);
        (c = this.get("name")) && this.fire("value-changed", {
          name: c,
          value: b.newVal,
          oldValue: b.prevVal,
          field: this
        })
      },
      this)
    }, this, "bindUI", this);
    b.Do.after(function() {
      a && a.defaultHidden && this.hide(!0)
    }, this, "syncUI", this)
  };
  c.NAME = "dialogFieldLegacyInterface";
  c.prototype = {
    temporaryHide: function() {
      this.hide()
    },
    temporaryShow: function() {
      this.show()
    },
    getName: function() {
      return this.get("name")
    },
    getType: function() {
      return this.name
    },
    getValue: function() {
      return this.get("data")
    },
    getErrors: function() {
      return this.get("errors")
    },
    getNode: function() {
      return this.get("boundingBox")
    },
    setValue: function(a) {
      this.set("data", a)
    },
    clearError: function() {
      this.hideError()
    }
  }
}, "1.0");
YUI.add("dd-drop", function(b, c) {
  var a = b.DD.DDM,
    d = function() {
      this._lazyAddAttrs = !1;
      d.superclass.constructor.apply(this, arguments);
      b.on("domready", b.bind(function() {
        b.later(100, this, this._createShim)
      }, this));
      a._regTarget(this)
    };
  d.NAME = "drop";
  d.ATTRS = {
    node: {
      setter: function(a) {
        var c = b.one(a);
        c || b.error("DD.Drop: Invalid Node Given: " + a);
        return c
      }
    },
    groups: {
      value: ["default"],
      getter: function() {
        return !this._groups ? (this._groups = {}, []) : b.Object.keys(this._groups)
      },
      setter: function(a) {
        this._groups = b.Array.hash(a);
        return a
      }
    },
    padding: {
      value: "0",
      setter: function(b) {
        return a.cssSizestoObject(b)
      }
    },
    lock: {
      value: !1,
      setter: function(b) {
        b ? this.get("node").addClass(a.CSS_PREFIX + "-drop-locked") : this.get("node").removeClass(a.CSS_PREFIX + "-drop-locked");
        return b
      }
    },
    bubbles: {
      setter: function(a) {
        this.addTarget(a);
        return a
      }
    },
    useShim: {
      value: !0,
      setter: function(a) {
        b.DD.DDM._noShim = !a;
        return a
      }
    }
  };
  b.extend(d, b.Base, {
    _bubbleTargets: b.DD.DDM,
    addToGroup: function(a) {
      this._groups[a] = !0;
      return this
    },
    removeFromGroup: function(a) {
      delete this._groups[a];
      return this
    },
    _createEvents: function() {
      b.Array.each(["drop:over", "drop:enter", "drop:exit", "drop:hit"], function(a) {
        this.publish(a, {
          type: a,
          emitFacade: !0,
          preventable: !1,
          bubbles: !0,
          queuable: !1,
          prefix: "drop"
        })
      }, this)
    },
    _valid: null,
    _groups: null,
    shim: null,
    region: null,
    overTarget: null,
    inGroup: function(a) {
      var c = this._valid = !1;
      b.Array.each(a, function(a) {
        this._groups[a] && (this._valid = c = !0)
      }, this);
      return c
    },
    initializer: function() {
      b.later(100, this, this._createEvents);
      var c = this.get("node"),
        d;
      c.get("id") || (d = b.stamp(c),
      c.set("id", d));
      c.addClass(a.CSS_PREFIX + "-drop");
      this.set("groups", this.get("groups"))
    },
    destructor: function() {
      a._unregTarget(this);
      this.shim && this.shim !== this.get("node") && (this.shim.detachAll(), this.shim.remove(), this.shim = null);
      this.get("node").removeClass(a.CSS_PREFIX + "-drop");
      this.detachAll()
    },
    _deactivateShim: function() {
      if (!this.shim) return !1;
      this.get("node").removeClass(a.CSS_PREFIX + "-drop-active-valid");
      this.get("node").removeClass(a.CSS_PREFIX + "-drop-active-invalid");
      this.get("node").removeClass(a.CSS_PREFIX + "-drop-over");
      this.get("useShim") && this.shim.setStyles({
        top: "-999px",
        left: "-999px",
        zIndex: "1"
      });
      this.overTarget = !1
    },
    _activateShim: function() {
      if (!a.activeDrag || this.get("node") === a.activeDrag.get("node") || this.get("lock")) return !1;
      var b = this.get("node");
      this.inGroup(a.activeDrag.get("groups")) ? (b.removeClass(a.CSS_PREFIX + "-drop-active-invalid"), b.addClass(a.CSS_PREFIX + "-drop-active-valid"), a._addValid(this), this.overTarget = !1, this.get("useShim") || (this.shim = this.get("node")), this.sizeShim()) : (a._removeValid(this),
      b.removeClass(a.CSS_PREFIX + "-drop-active-valid"), b.addClass(a.CSS_PREFIX + "-drop-active-invalid"))
    },
    sizeShim: function() {
      if (!a.activeDrag || this.get("node") === a.activeDrag.get("node") || this.get("lock")) return !1;
      if (!this.shim) return b.later(100, this, this.sizeShim), !1;
      var c = this.get("node"),
        d = c.get("offsetHeight"),
        g = c.get("offsetWidth"),
        c = c.getXY(),
        h = this.get("padding"),
        k, l, g = g + h.left + h.right,
        d = d + h.top + h.bottom;
      c[0] -= h.left;
      c[1] -= h.top;
      a.activeDrag.get("dragMode") === a.INTERSECT && (h = a.activeDrag, k = h.get("node").get("offsetHeight"),
      l = h.get("node").get("offsetWidth"), d += k, g += l, c[0] -= l - h.deltaXY[0], c[1] -= k - h.deltaXY[1]);
      this.get("useShim") && this.shim.setStyles({
        height: d + "px",
        width: g + "px",
        top: c[1] + "px",
        left: c[0] + "px"
      });
      this.region = {
        0: c[0],
        1: c[1],
        area: 0,
        top: c[1],
        right: c[0] + g,
        bottom: c[1] + d,
        left: c[0]
      }
    },
    _createShim: function() {
      if (a._pg) {
        if (!this.shim) {
          var c = this.get("node");
          this.get("useShim") && (c = b.Node.create('<div id="' + this.get("node").get("id") + '_shim"></div>'), c.setStyles({
            height: this.get("node").get("offsetHeight") + "px",
            width: this.get("node").get("offsetWidth") + "px",
            backgroundColor: "yellow",
            opacity: ".5",
            zIndex: "1",
            overflow: "hidden",
            top: "-900px",
            left: "-900px",
            position: "absolute"
          }), a._pg.appendChild(c), c.on("mouseover", b.bind(this._handleOverEvent, this)), c.on("mouseout", b.bind(this._handleOutEvent, this)));
          this.shim = c
        }
      } else b.later(10, this, this._createShim)
    },
    _handleTargetOver: function() {
      a.isOverTarget(this) ? (this.get("node").addClass(a.CSS_PREFIX + "-drop-over"), a.activeDrop = this, a.otherDrops[this] = this, this.overTarget ? (a.activeDrag.fire("drag:over", {
        drop: this,
        drag: a.activeDrag
      }), this.fire("drop:over", {
        drop: this,
        drag: a.activeDrag
      })) : a.activeDrag.get("dragging") && (this.overTarget = !0, this.fire("drop:enter", {
        drop: this,
        drag: a.activeDrag
      }), a.activeDrag.fire("drag:enter", {
        drop: this,
        drag: a.activeDrag
      }), a.activeDrag.get("node").addClass(a.CSS_PREFIX + "-drag-over"))) : this._handleOut()
    },
    _handleOverEvent: function() {
      this.shim.setStyle("zIndex", "999");
      a._addActiveShim(this)
    },
    _handleOutEvent: function() {
      this.shim.setStyle("zIndex", "1");
      a._removeActiveShim(this)
    },
    _handleOut: function(b) {
      if ((!a.isOverTarget(this) || b) && this.overTarget) this.overTarget = !1, b || a._removeActiveShim(this), a.activeDrag && (this.get("node").removeClass(a.CSS_PREFIX + "-drop-over"), a.activeDrag.get("node").removeClass(a.CSS_PREFIX + "-drag-over"), this.fire("drop:exit", {
        drop: this,
        drag: a.activeDrag
      }), a.activeDrag.fire("drag:exit", {
        drop: this,
        drag: a.activeDrag
      }), delete a.otherDrops[this])
    }
  });
  b.DD.Drop = d
}, "3.17.2", {
  requires: ["dd-drag", "dd-ddm-drop"]
});
YUI.add("squarespace-checkout-coupon-list", function(b) {
  b.namespace("Squarespace.Widgets");
  b.Squarespace.Widgets.CheckoutCouponList = b.Base.create("checkoutCouponList", b.Squarespace.SSWidget, [], {
    renderUI: function() {
      b.Squarespace.Widgets.CheckoutCouponList.superclass.renderUI.call(this);
      var c = this.get("contentBox");
      this._validList = new b.Squarespace.DialogFields.List({
        listItemConstructor: b.Squarespace.DialogFields.CheckoutCouponListItem,
        showAddControl: !1,
        "strings.emptyText": "",
        render: c.one(".coupon-list")
      });
      this._invalidList = new b.Squarespace.DialogFields.List({
        listItemConstructor: b.Squarespace.DialogFields.CheckoutCouponListItem,
        showAddControl: !1,
        "strings.emptyText": "",
        render: c.one(".invalid-coupon-list")
      })
    },
    bindUI: function() {
      b.Squarespace.Widgets.CheckoutCouponList.superclass.bindUI.call(this);
      var c = this.get("contentBox"),
        a = this.get("model");
      a.on("change", this.syncUI, this);
      c.one(".redeem-coupon").on("click", this._submitPromoCode, this);
      c.one('input[name="promoCode"]').on("keydown", function(a) {
        13 === a.keyCode && this._submitPromoCode()
      }, this);
      c.delegate("click", function(b) {
        a.removeCoupon(b.target.getData("coupon-id"))
      }, ".remove-coupon", this)
    },
    syncUI: function() {
      this._validList.clearItems();
      this._invalidList.clearItems();
      var c = this.get("contentBox"),
        a = this.get("model"),
        d = a.get("validCoupons"),
        a = a.get("invalidCoupons"),
        e = c.one(".invalid-coupon-title"),
        c = c.one(".invalid-coupon-list");
      0 < a.length ? (e.removeClass("hidden"), c.removeClass("hidden")) : (e.addClass("hidden"), c.addClass("hidden"));
      b.Array.each(d,

      function(a) {
        this._validList.addItem(a)
      }, this);
      b.Array.each(a, function(a) {
        this._invalidList.addItem(a)
      }, this)
    },
    lock: function() {
      var b = this.get("contentBox");
      b.addClass("locked");
      b.one("input").set("disabled", !0)
    },
    unlock: function() {
      var b = this.get("contentBox");
      b.removeClass("locked");
      b.one("input").set("disabled", !1)
    },
    _onItemClick: function(b) {
      b.originalClickEvent.target.hasClass("remove-coupon") && this.get("model").removeCoupon(b.fieldData.id)
    },
    _submitPromoCode: function() {
      var c = this.get("contentBox").one('input[name="promoCode"]'),
        a = c.get("value");
      0 !== a.length && this.get("model").addCoupon(a, function(a) {
        if (a)(new b.Squarespace.Widgets.Alert({
          "strings.title": "Unable to Redeem Coupon",
          "strings.message": a
        })).on("confirm", function() {
          c.focus()
        });
        else c.set("value", "")
      }, this)
    }
  }, {
    HANDLEBARS_TEMPLATE: "checkout-coupon-list.html",
    CSS_PREFIX: "sqs-checkout-coupon-list",
    ATTRS: {
      model: {
        value: null,
        validator: function(c) {
          return b.instanceOf(c, b.Squarespace.Models.ShoppingCart)
        }
      }
    }
  })
}, "1.0", {
  requires: "squarespace-commerce-utils squarespace-list squarespace-models-shopping-cart squarespace-checkout-coupon-list-template squarespace-dialog-checkout-coupon-list-item squarespace-widgets-alert".split(" ")
});
YUI.add("datatable-column-widths", function(b, c) {
  function a() {}
  var d = b.Lang.isNumber,
    e = b.Array.indexOf;
  b.Features.add("table", "badColWidth", {
    test: function() {
      var a = b.one("body"),
        c;
      a && (a = a.insertBefore('<table style="position:absolute;visibility:hidden;border:0 none"><colgroup><col style="width:9px"></colgroup><tbody><tr><td style="padding:0 4px;font:normal 2px/2px arial;border:0 none">.</td></tr></tbody></table>', a.get("firstChild")), c = "1px" !== a.one("td").getComputedStyle("width"), a.remove(!0));
      return c
    }
  });
  b.mix(a.prototype, {
    COL_TEMPLATE: "<col/>",
    COLGROUP_TEMPLATE: "<colgroup/>",
    setColumnWidth: function(a, b) {
      var c = this.getColumn(a),
        k = c && e(this._displayColumns, c); - 1 < k && (d(b) && (b += "px"), c.width = b, this._setColumnWidth(k, b));
      return this
    },
    _createColumnGroup: function() {
      return b.Node.create(this.COLGROUP_TEMPLATE)
    },
    initializer: function() {
      this.after(["renderView", "columnsChange"], this._uiSetColumnWidths)
    },
    _setColumnWidth: function(a, c) {
      var e = this._colgroupNode,
        e = e && e.all("col").item(a),
        k, l;
      if (e && (c && d(c) && (c += "px"), e.setStyle("width", c), c && b.Features.test("table", "badColWidth") && (k = this.getCell([0, a])))) l = function(a) {
        return parseInt(k.getComputedStyle(a), 10) || 0
      }, e.setStyle("width", parseInt(c, 10) - l("paddingLeft") - l("paddingRight") - l("borderLeftWidth") - l("borderRightWidth") + "px")
    },
    _uiSetColumnWidths: function() {
      if (this.view) {
        var a = this.COL_TEMPLATE,
          b = this._colgroupNode,
          c = this._displayColumns,
          d, e;
        b ? b.empty() : (b = this._colgroupNode = this._createColumnGroup(), this._tableNode.insertBefore(b, this._tableNode.one("> thead, > tfoot, > tbody")));
        d = 0;
        for (e = c.length; d < e; ++d) b.append(a), this._setColumnWidth(d, c[d].width)
      }
    }
  }, !0);
  b.DataTable.ColumnWidths = a;
  b.Base.mix(b.DataTable, [a])
}, "3.17.2", {
  requires: ["datatable-base"]
});
YUI.add("squarespace-dialog-checkout-coupon-list-item", function(b) {
  b.namespace("Squarespace.DialogFields");
  b.Squarespace.DialogFields.CheckoutCouponListItem = b.Base.create("checkoutCouponListItem", b.Squarespace.Widgets.DialogField2, [], {
    syncUI: function() {
      var c = this.get("data");
      this.get("contentBox").setHTML('<div class="remove-coupon" data-coupon-id="' + c.id + '"></div><div class="coupon-amount">- ' + b.Squarespace.Commerce.moneyString(c.computedDiscount) + '</div><div class="coupon-title"><strong>' + c.name + "</strong> (" + c.promoCode + ')</div><div class="coupon-summary">' + b.Squarespace.CommerceCouponFormatters.getCouponSummary(c) + "</div>")
    }
  }, {
    CSS_PREFIX: "sqs-checkout-coupon-list-item"
  })
}, 1, {
  requires: ["base", "node", "squarespace-dialog-field-2", "squarespace-commerce-utils", "squarespace-commerce-coupon-formatters"]
});
YUI.add("squarespace-dialog-fields", function(b) {
  b.namespace("Squarespace");
  b.Squarespace.DialogField = Class.extend(b.Squarespace.Gizmo, {
    _name: "DialogField",
    initialize: function(c, a, d) {
      this._super();
      this.config = b.merge({
        defaultHidden: !1
      }, c);
      this.dialog = this.panel = d;
      this.initialData = b.merge({}, a);
      this.className = "";
      this.inActiveFrame = !0;
      this.config && ("undefined" !== typeof this.config.name && this.config.name in a) && (this.value = a[this.config.name])
    },
    hide: function(b) {
      this.temporaryHide(b)
    },
    getTakenHeight: function() {
      return this.html.get("offsetHeight")
    },
    show: function(b) {
      this.temporaryShow(b)
    },
    getName: function() {
      return this.config.name
    },
    getType: function() {
      return this.type
    },
    getDialog: function() {
      return this.dialog
    },
    append: function(b) {
      b.append(this.html);
      this.config.defaultHidden && this.temporaryHide(!1)
    },
    resize: function() {},
    isValid: function() {
      return 0 < this.getErrors().length
    },
    getErrors: function() {
      return this.config.validator && this.config.validationErrorMsg && !this.config.validator.call(this, this.dialog) ? [this.config.validationErrorMsg] : []
    },
    showError: function(c) {
      if (c) {
        var a = this.errorFlyoutAnchor || this.control;
        a || console.error("dialog-field: [DialogField] No control or error flyout anchor set to throw form field error on. Set this.control or this.errorFlyoutAnchor to a node.");
        a.hasPlugin("flyoutPlugin") || a.plug(b.Squarespace.Animations.Flyout, {
          duration: 0.3
        });
        a = a.flyoutPlugin;
        a.get("visible") ? (this._showErrorSub && this._showErrorSub.detach(), this._showErrorSub = a.once(this.getId() + "|hidden", function() {
          this._showErrorSub = null;
          this._doShowErrorFlyout(c)
        }, this), a.hide()) : this._doShowErrorFlyout(c)
      }
    },
    _doShowErrorFlyout: function(c) {
      var a = this.errorFlyoutAnchor || this.control,
        d = a.flyoutPlugin;
      this.html.addClass("error");
      this._errorEl || (this._errorEl = b.DB.DIV("flyout-error-message", {
        html: c
      }), this._errorEl.setStyle("zIndex", this.dialog.zIndex + 10), this._errorEl.on(this.getId() + "|click", function(a) {
        a.halt()
      }, this));
      this._errorEl.setContent(c);
      c = b.Squarespace.Utils.measureNode(this._errorEl).width;
      a = a.getX() + a.get("offsetWidth") - (this.html.hasClass("thin") ? -10 : 1) + c;
      c = b.one(window).get("region").right;
      (a = a > c) && this._errorEl.addClass("out-from-left");
      d.setAttrs({
        node: this._errorEl,
        alignment: a ? "lt" : "rt"
      });
      this._clearErrorSub = b.on(this.getId() + "|click", this.clearError, this.errorEl, this);
      d.show()
    },
    hideError: function() {
      var b = this.errorFlyoutAnchor || this.control;
      b && (b.hasPlugin("flyoutPlugin") && b.flyoutPlugin.get("visible")) && (this._clearErrorSub && this._clearErrorSub.detach(), b = b.flyoutPlugin, this._subscribeOnce(b, this.getId() + "|hidden", function(a) {
        this.html.removeClass("error")
      }, this), b.hide())
    },
    clearError: function() {
      this.dialog.clearError(this);
      this.hideError()
    },
    scrollIntoView: function() {
      this.html.scrollIntoView()
    },
    updateInlineTitle: function() {},
    setHeight: function(b) {
      this.control && this.control.setStyle("height", b + "px")
    },
    getValue: function() {
      return this.value
    },
    setValue: function(b) {
      var a = this.value;
      this.value = b;
      "config" in this && "name" in this.config && this.fire("value-changed", {
        name: this.config.name,
        value: b,
        oldValue: a,
        field: this
      })
    },
    temporaryHide: function(c) {
      this.fire("hide", this);
      this.hidden = !0;
      this.hideAnim && this.hideAnim.stop();
      c ? (this.html.setStyle("display", "none"), this.fire("hidden", this)) : (this.hideAnim = this._anim({
        node: this.html,
        to: {
          opacity: 0
        },
        duration: 0.35,
        easing: b.Easing.easeOutStrong
      }), this.hideAnim.on("end", function() {
        this.html.setStyle("display", "none");
        this.fire("hidden", this)
      }, this), this.hideAnim.run())
    },
    temporaryShow: function() {
      this.fire("show", this);
      this.hidden = !1;
      this.hideAnim && this.hideAnim.stop();
      this.html.setStyle("display", "block");
      this.hideAnim = this._anim({
        node: this.html,
        to: {
          opacity: 1
        },
        duration: 0.35,
        easing: b.Easing.easeOutStrong
      });
      this.hideAnim.on("end", function() {
        b.fire("shown", this);
        b.fire("showing", this);
        this.fire("shown")
      }, this);
      this.hideAnim.run()
    },
    getNode: function() {
      return this.html
    },
    didDataChange: function() {
      return this.config.ignoreChanges ? !1 : !this.initialData || b.Lang.isUndefined(this.initialData[this.getName()]) && "" === this.getValue() ? !1 : b.Lang.isArray(this.initialData[this.getName()]) ? b.JSON.stringify(this.initialData[this.getName()]) !== b.JSON.stringify(this.getValue()) : this.getValue() !== this.initialData[this.getName()]
    }
  })
}, "1.0", {
  requires: "node datatype-date node-focusmanager anim dd attribute slider datatable json widget node-event-simulate calendar squarespace-gizmo squarespace-dombuilder squarespace-debugger squarespace-toggle squarespace-node-flyout squarespace-structured-input squarespace-mailcheck squarespace-util".split(" ")
});
YUI.add("datatable-body", function(b, c) {
  var a = b.Lang,
    d = a.isArray,
    e = a.isNumber,
    f = a.isString,
    g = a.sub,
    h = b.Escape.html,
    k = b.Array,
    l = b.bind,
    m = b.Object,
    n = /\{value\}/g,
    p = {
      above: [-1, 0],
      below: [1, 0],
      next: [0, 1],
      prev: [0, - 1],
      previous: [0, - 1]
    };
  b.namespace("DataTable").BodyView = b.Base.create("tableBody", b.View, [], {
    CELL_TEMPLATE: '<td {headers} class="{className}">{content}</td>',
    ROW_TEMPLATE: '<tr id="{rowId}" data-yui3-record="{clientId}" class="{rowClass}">{content}</tr>',
    TBODY_TEMPLATE: '<tbody class="{className}"></tbody>',
    getCell: function(a, c) {
      var e = this.tbodyNode,
        g, h, k;
      a && e && (d(a) ? h = (g = e.get("children").item(a[0])) && g.get("children").item(a[1]) : a._node && (h = a.ancestor("." + this.getClassName("cell"), !0)), h && c && (g = e.get("firstChild.rowIndex"), f(c) && (p[c] || b.error("Unrecognized shift: " + c, null, "datatable-body"), c = p[c]), d(c) && (k = h.get("parentNode.rowIndex") + c[0] - g, g = e.get("children").item(k), k = h.get("cellIndex") + c[1], h = g && g.get("children").item(k))));
      return h || null
    },
    getClassName: function() {
      var a = this.host;
      if (a && a.getClassName) return a.getClassName.apply(a,
      arguments);
      a = k(arguments);
      a.unshift(this.constructor.NAME);
      return b.ClassNameManager.getClassName.apply(b.ClassNameManager, a)
    },
    getRecord: function(a) {
      var b = this.get("modelList"),
        c = this.tbodyNode,
        d = null,
        e;
      c && (f(a) && (a = c.one("#" + a)), a && a._node && (e = (d = a.ancestor(function(a) {
        return a.get("parentNode").compareTo(c)
      }, !0)) && b.getByClientId(d.getData("yui3-record"))));
      return e || null
    },
    getRow: function(a) {
      var b = this.tbodyNode,
        c = null;
      b && (a && (a = this._idMap[a.get ? a.get("clientId") : a] || a), c = e(a) ? b.get("children").item(a) : b.one("#" + a));
      return c
    },
    render: function() {
      var a = this.get("container"),
        b = this.get("modelList"),
        c = this.get("columns"),
        d = this.tbodyNode || (this.tbodyNode = this._createTBodyNode());
      this._createRowTemplate(c);
      b && (d.setHTML(this._createDataHTML(c)), this._applyNodeFormatters(d, c));
      d.get("parentNode") !== a && a.appendChild(d);
      this.bindUI();
      return this
    },
    refreshRow: function(a, b, c) {
      var d, e = c.length,
        f;
      for (f = 0; f < e; f++) d = this.getColumn(c[f]), null !== d && (d = a.one("." + this.getClassName("col", d._id || d.key)), this.refreshCell(d,
      b));
      return this
    },
    refreshCell: function(a, c, d) {
      var e, f, g = c.toJSON();
      a = this.getCell(a);
      c || (c = this.getRecord(a));
      d || (d = this.getColumn(a));
      if (d.nodeFormatter) c = {
        cell: a.one("." + this.getClassName("liner")) || a,
        column: d,
        data: g,
        record: c,
        rowIndex: this._getRowIndex(a.ancestor("tr")),
        td: a,
        value: g[d.key]
      }, keep = d.nodeFormatter.call(host, c), !1 === keep && a.destroy(!0);
      else if (d.formatter) {
        d._formatterFn || (d = this._setColumnsFormatterFn([d])[0]);
        if (f = d._formatterFn || null) c = {
          value: g[d.key],
          data: g,
          column: d,
          record: c,
          className: "",
          rowClass: "",
          rowIndex: this._getRowIndex(a.ancestor("tr"))
        }, e = f.call(this.get("host"), c), void 0 === e && (e = c.value);
        if (void 0 === e || null === e || "" === e) e = d.emptyCellValue || ""
      } else e = g[d.key] || d.emptyCellValue || "";
      a.setHTML(d.allowHTML ? e : b.Escape.html(e));
      return this
    },
    getColumn: function(a) {
      a && a._node && (a = a.get("className").match(RegExp(this.getClassName("col") + "-([^ ]*)"))[1]);
      if (this.host) return this.host._columnMap[a] || null;
      var c = this.get("columns"),
        d = null;
      b.Array.some(c, function(b) {
        if ((b._id || b.key) === a) return d = b, !0
      });
      return d
    },
    _afterColumnsChange: function() {
      this.render()
    },
    _afterDataChange: function(a) {
      var c = (a.type.match(/:(add|change|remove)$/) || [])[1],
        d = a.index,
        e = this.get("columns"),
        f, g = a.changed && b.Object.keys(a.changed),
        h, k;
      h = 0;
      for (k = e.length; h < k; h++) if (f = e[h], f.hasOwnProperty("nodeFormatter")) {
        this.render();
        this.fire("contentUpdate");
        return
      }
      switch (c) {
      case "change":
        h = 0;
        for (k = e.length; h < k; h++) f = e[h], d = f.key, f.formatter && !a.changed[d] && g.push(d);
        this.refreshRow(this.getRow(a.target), a.target, g);
        break;
      case "add":
        d = Math.min(d, this.get("modelList").size() - 1);
        this._setColumnsFormatterFn(e);
        a = b.Node.create(this._createRowHTML(a.model, d, e));
        this.tbodyNode.insert(a, d);
        this._restripe(d);
        break;
      case "remove":
        this.getRow(d).remove(!0);
        this._restripe(d - 1);
        break;
      default:
        this.render()
      }
      this.fire("contentUpdate")
    },
    _restripe: function(a) {
      var b = this._restripeTask,
        c;
      a = Math.max(a | 0, 0);
      b ? b.index = Math.min(b.index, a) : (c = this, this._restripeTask = {
        timer: setTimeout(function() {
          if (c && !c.get("destroy") && c.tbodyNode && c.tbodyNode.inDoc()) {
            var a = [c.CLASS_ODD, c.CLASS_EVEN],
              b = [c.CLASS_EVEN, c.CLASS_ODD],
              d = c._restripeTask.index;
            c.tbodyNode.get("childNodes").slice(d).each(function(c, e) {
              c.replaceClass.apply(c, (d + e) % 2 ? b : a)
            })
          }
          c._restripeTask = null
        }, 0),
        index: a
      })
    },
    _afterModelListChange: function() {
      var a = this._eventHandles;
      a.dataChange && (a.dataChange.detach(), delete a.dataChange, this.bindUI());
      this.tbodyNode && this.render()
    },
    _applyNodeFormatters: function(a, b) {
      var c = this.host || this,
        d = this.get("modelList"),
        e = [],
        f = "." + this.getClassName("liner"),
        g, h, k;
      h = 0;
      for (k = b.length; h < k; ++h) b[h].nodeFormatter && e.push(h);
      d && e.length && (g = a.get("childNodes"), d.each(function(a, d) {
        var h = {
          data: a.toJSON(),
          record: a,
          rowIndex: d
        }, k = g.item(d),
          l, m, p, n, s;
        if (k) {
          n = k.get("childNodes");
          k = 0;
          for (l = e.length; k < l; ++k) if (s = n.item(e[k])) m = h.column = b[e[k]], p = m.key || m.id, h.value = a.get(p), h.td = s, h.cell = s.one(f) || s, m = m.nodeFormatter.call(c, h), !1 === m && s.destroy(!0)
        }
      }))
    },
    bindUI: function() {
      var a = this._eventHandles,
        b = this.get("modelList"),
        c = b.model.NAME + ":change";
      a.columnsChange || (a.columnsChange = this.after("columnsChange", l("_afterColumnsChange", this)));
      b && !a.dataChange && (a.dataChange = b.after(["add", "remove", "reset", c], l("_afterDataChange", this)))
    },
    _createDataHTML: function(a) {
      var b = this.get("modelList"),
        c = "";
      b && b.each(function(b, d) {
        c += this._createRowHTML(b, d, a)
      }, this);
      return c
    },
    _createRowHTML: function(a, b, c) {
      var d = a.toJSON(),
        e = a.get("clientId"),
        e = {
          rowId: this._getRowId(e),
          clientId: e,
          rowClass: b % 2 ? this.CLASS_ODD : this.CLASS_EVEN
        }, f = this.host || this,
        k, l, m, p, n, A;
      k = 0;
      for (l = c.length; k < l; ++k) if (m = c[k], n = d[m.key], p = m._id || m.key, e[p + "-className"] = "", m._formatterFn && (A = {
        value: n,
        data: d,
        column: m,
        record: a,
        className: "",
        rowClass: "",
        rowIndex: b
      }, n = m._formatterFn.call(f, A), void 0 === n && (n = A.value), e[p + "-className"] = A.className, e.rowClass += " " + A.rowClass), !e.hasOwnProperty(p) || d.hasOwnProperty(m.key)) {
        if (void 0 === n || null === n || "" === n) n = m.emptyCellValue || "";
        e[p] = m.allowHTML ? n : h(n)
      }
      e.rowClass = e.rowClass.replace(/\s+/g, " ");
      return g(this._rowTemplate, e)
    },
    _getRowIndex: function(a) {
      var b = this.tbodyNode,
        c = 1;
      if (b && a) {
        if (a.ancestor("tbody") !== b) return null;
        for (; a = a.previous();) c++
      }
      return c
    },
    _createRowTemplate: function(a) {
      var b = "",
        c = this.CELL_TEMPLATE,
        d, e, f, h, k, l;
      this._setColumnsFormatterFn(a);
      d = 0;
      for (e = a.length; d < e; ++d) f = a[d], h = f.key, k = f._id || h, h = f._formatterFn, l = 1 < (f._headers || []).length ? 'headers="' + f._headers.join(" ") + '"' : "", k = {
        content: "{" + k + "}",
        headers: l,
        className: this.getClassName("col", k) + " " + (f.className || "") + " " + this.getClassName("cell") + " {" + k + "-className}"
      }, !h && f.formatter && (k.content = f.formatter.replace(n,
      k.content)), f.nodeFormatter && (k.content = ""), b += g(f.cellTemplate || c, k);
      this._rowTemplate = g(this.ROW_TEMPLATE, {
        content: b
      })
    },
    _setColumnsFormatterFn: function(c) {
      var d = b.DataTable.BodyView.Formatters,
        e, f, g, h;
      g = 0;
      for (h = c.length; g < h; g++) f = c[g], e = f.formatter, !f._formatterFn && e && (a.isFunction(e) ? f._formatterFn = e : e in d && (f._formatterFn = d[e].call(this.host || this, f)));
      return c
    },
    _createTBodyNode: function() {
      return b.Node.create(g(this.TBODY_TEMPLATE, {
        className: this.getClassName("data")
      }))
    },
    destructor: function() {
      (new b.EventHandle(m.values(this._eventHandles))).detach()
    },
    _getRowId: function(a) {
      return this._idMap[a] || (this._idMap[a] = b.guid())
    },
    initializer: function(a) {
      this.host = a.host;
      this._eventHandles = {
        modelListChange: this.after("modelListChange", l("_afterModelListChange", this))
      };
      this._idMap = {};
      this.CLASS_ODD = this.getClassName("odd");
      this.CLASS_EVEN = this.getClassName("even")
    }
  }, {
    Formatters: {}
  })
}, "3.17.2", {
  requires: ["datatable-core", "view", "classnamemanager"]
});
YUI.add("squarespace-list", function(b) {
  var c = b.namespace("Squarespace.DialogFields").List = b.Base.create("list", b.Squarespace.DialogField2, [], {
    initializer: function(a) {
      var b = this.getProperty("DATA_STATES");
      (a = a.data) ? (this.set("dataState", this.getProperty("DATA_STATES").LOADED), this._addItems(a)) : (this.after(this.get("id") + "|dataStateChange", function(a) {
        a.newVal === b.LOADED && this._addItems(a.data || this.get("data"))
      }, this), this._loadData())
    },
    renderUI: function() {
      c.superclass.renderUI.call(this);
      var a = this.get("boundingBox"),
        b = this.get("strings");
      a.one("button.add-item").setContent(b.addButton);
      a.one(".list-title").setContent(b.title);
      b = a.one(".list-items .empty");
      b.setContent(this.get("strings.emptyText"));
      b.show();
      b = a.one(".add-item");
      this.get("showAddControl") ? b.show() : b.hide();
      this._childrenContainer = a.one(".list-items")
    },
    bindUI: function() {
      c.superclass.bindUI.call(this);
      var a = this.get("boundingBox");
      a.on(this.get("id") + "|click", function(a) {
        if (a.target.ancestor(".list-items", !1)) {
          var c = a.target.ancestor("." + b.Squarespace.DialogField2.CSS_PREFIX, !0).getAttribute("id"),
            c = this.getFieldById(c);
          null === c ? console.warn(this.name + ": Field widget not found, no event will fire.") : this.fire("list-item-click", {
            originalClickEvent: a,
            field: c,
            fieldData: c.get("data")
          })
        }
      }, this);
      a.one("button.add-item").on(this.get("id") + "|click", function() {
        this.fire("add-requested")
      }, this)
    },
    syncUI: function() {
      c.superclass.syncUI.call(this);
      var a = this.get("data"),
        b = this.get("boundingBox").one(".list-items").one(".empty");
      this.clearItems();
      a && a.length ? (this._addItems(a), b.hide()) : b.show()
    },
    getFieldById: function(a) {
      var b = null;
      this.some(function(c) {
        return c.get("boundingBox").get("id") === a ? (b = c, !0) : !1
      });
      return b
    },
    _addItems: function(a) {
      if (!b.Lang.isArray(a)) throw this.name + ": List data was not an array";
      b.Array.each(a, function(a) {
        this._addItem(a)
      }, this)
    },
    _addItem: function(a) {
      this.get("rendered") && this.get("boundingBox").one(".list-items .empty").hide();
      a = this.add({
        childType: this.get("listItemConstructor"),
        data: a
      }, 0).item(0);
      if (!a) throw this.name + ": Creating list item failed.";
      a.after(this.get("id") + "|dataChange", this._updateItem, this);
      a.after("destroy", function(a) {
        this._removeItem(a.target)
      }, this);
      return a
    },
    addItem: function(a) {
      var c = this._addItem(a),
        e = this._parseURLObject(this.get("apiURLs.createURL")),
        f = e.url,
        g = e.responseDataProperty;
      f && b.Data.post({
        url: f,
        data: a,
        success: function(e) {
          this.get("mergeResponseData") && c.set("data", b.merge(a, g ? e[g] : e), {
            silent: !0
          })
        },
        failure: function(a) {
          console.warn(this.name + ": Failed to save item, reverting.");
          c.destroy()
        }
      }, this);
      return c
    },
    removeItem: function(a) {
      -1 === this.indexOf(a) ? console.warn(this.name + ": That item is not in this list") : a.destroy()
    },
    _removeItem: function(a) {
      var c = a.get("boundingBox");
      c.remove();
      var e = this._parseURLObject(this.get("apiURLs.deleteURL")).url,
        f = b.bind(function() {
          a.get("destroyed") || this.remove(this.indexOf(a));
          this.isEmpty() && this.get("contentBox").one(".list-items .empty").show()
        }, this);
      e ? b.Data.post({
        url: e,
        data: a.get("data"),
        success: f,
        failure: function() {
          c.appendTo(this._childrenContainer)
        }
      },
      this) : f()
    },
    clearItems: function() {
      for (; 0 < this.size();) this._removeItem(this.item(0), !1);
      this.get("boundingBox").one(".list-items .empty").show()
    },
    _loadData: function() {
      var a = this.getProperty("DATA_STATES");
      this.set("dataState", a.LOADING);
      var c = this.get("apiURLs");
      if (!c || !this._parseURLObject(c.readURL).url) this.set("dataState", a.LOADED);
      else {
        var c = this._parseURLObject(c.readURL),
          e = c.responseDataProperty,
          f = c.responseDataSanitizer;
        b.Data.get({
          url: c.url,
          data: this.get("requestData") || {},
          success: function(c) {
            b.Lang.isFunction(f) && (c = f(c));
            c = e ? c[e] : c;
            this.set("data", c);
            this.set("dataState", a.LOADED, {
              data: c
            })
          },
          failure: function() {
            this.get("boundingBox").one(".list-items").setContent(this.get("strings.loadFail"));
            this.set("dataState", a.LOAD_FAILED)
          }
        }, this)
      }
    },
    _parseURLObject: function(a) {
      var c = {
        url: null,
        requestData: null,
        responseDataProperty: null,
        responseDataSanitizer: null
      };
      return !a ? c : b.Lang.isString(a) ? b.merge(c, {
        url: a || null
      }) : b.merge(c, {
        url: a.url || null,
        requestData: a.requestData || null,
        responseDataProperty: a.responseDataProperty || null,
        responseDataSanitizer: a.responseDataSanitizer || null
      })
    },
    _updateItem: function(a) {
      var c = this._parseURLObject(this.get("apiURLs.updateURL")).url;
      if (c && !a.silent) {
        var e = a.target,
          f = e.get("data");
        b.Data.post({
          url: c,
          data: f,
          success: function(a) {
            this.get("mergeResponseData") && (a = this.get("mergeResponseDataProperty"), e.set("data", b.merge(f, a ? responseData[a] : responseData), {
              silent: !0
            }))
          },
          failure: function(b) {
            e.set("data", a.prevVal, {
              silent: !0
            })
          }
        }, this)
      }
    }
  }, {
    CSS_PREFIX: "sqs-list",
    ASYNC_DATA: !0,
    TEMPLATE: '<div class="controls"><button class="add-item" type="button"></button></div><div class="list-title"></div><div class="list-items"><div class="empty"></div></div>',
    ATTRS: {
      strings: {
        value: {
          title: "Set strings.title to something.",
          addButton: "Add Item",
          emptyText: "No items.",
          loadFail: "Unable to load items."
        }
      },
      data: {
        value: [],
        getter: function() {
          var a = [];
          this.each(function(b) {
            a.push(b.get("data"))
          }, this);
          return a
        }
      },
      apiURLs: {
        value: {
          createURL: null,
          readURL: null,
          updateURL: null,
          deleteURL: null
        }
      },
      listItemConstructor: {
        value: b.Squarespace.DialogField2
      },
      requestData: {
        value: null
      },
      responseDataProperty: {
        value: null
      },
      mergeResponseData: {
        value: !0,
        validator: b.Lang.isBoolean
      },
      mergeResponseDataProperty: {
        value: null
      },
      showAddControl: {
        value: !0
      }
    }
  })
}, "1.0", {
  requires: ["widget-child", "widget-parent", "squarespace-dialog", "squarespace-dialog-field-2"]
});
YUI.add("squarespace-checkout", function(b) {}, "1.0", {
  requires: "squarespace-models-shopping-cart squarespace-checkout-form squarespace-checkout-shopping-cart squarespace-checkout-coupon-list squarespace-commerce-analytics squarespace-donate-form squarespace-donate-form-billing squarespace-contribution-summary squarespace-localities squarespace-modal-lightbox".split(" ")
});
