function setupStats() {
  $.getJSON("https://s3-us-west-1.amazonaws.com/skully/stats.json", function(a) {
    $("span.pledged-amount").text(a.funding);
    $("span.indiegogo-pledges").text(a.funders)
  });
  var b = "",
    a = new Date(1412924399E3) - new Date,
    c = 864E5 < a ? !0 : !1,
    a = a / 1E3 / 60,
    e = Math.floor(a % 60),
    a = a / 60,
    d = Math.floor(a % 24),
    a = a / 24;
  c && (b += "1 day, ");
  $("span.campaign-counter").text(b + (d + " hours, " + e + " minutes left to order your AR-1"))
}
Y.use("node", "squarespace-ui-base", function() {
  window.Site = Singleton.create({
    PARALLAX_FACTOR: 0.8,
    SCROLL_SPEED: 0.6,
    IMAGE_VIEWPORT: null,
    pageOffsets: {},
    docHeight: 0,
    ready: function() {
      Y.on("domready", this.initialize, this)
    },
    initialize: function() {
      setupStats();
      this.parallaxImages = Y.all("#parallax-images .image-container");
      this.parallaxPages = Y.all(".parallax-item");
      this.scrollEl = Y.one(Y.UA.gecko || Y.UA.ie || navigator.userAgent.match(/Trident.*rv.11\./) ? "html" : "body");
      this.viewportH = Y.one("body").get("winHeight");
      this.isMobile = !Y.Lang.isUndefined(window.orientation) || 0 < Y.UA.ie && 9 >= Y.UA.ie;
      this.initAnnouncementBar();
      this.bindUI();
      this.syncUI();
      this.textShrink(".page-title", ".title-desc-inner");
      this.textShrink(".collection-type-events.view-list .entry-title-wrapper h1.entry-title", ".collection-type-events.view-list .entry-title-wrapper");
      this.textShrink(".collection-type-blog.view-list.blog-layout-columns .entry-title-wrapper h1.entry-title", ".collection-type-blog.view-list.blog-layout-columns .entry-title-wrapper");
      Y.one("body.collection-type-index") && this.handleIndex();
      this.listenTweaks();
      Y.one("body").addClass("loaded");
      Y.UA.ie && Y.one("html").addClass("ie" + Y.UA.ie);
      this.isMobile || Y.one("html").removeClass("touch")
    },
    handleIndex: function() {
      if (window.location.hash) this.onHashChange({
        newHash: window.location.hash.replace("#", ""),
        quick: !0
      });
      else this.updateActivePage();
      this.positionImages()
    },
    bindUI: function() {
      if (Y.one("body.collection-type-index")) {
        Y.one(Y.config.win).on("scroll", Y.throttle(Y.bind(function() {
          this.positionImages();
          this.updateActivePage()
        }, this), 2), this);
        var b = Y.UA.mobile ? "orientationchange" : "resize";
        Y.one(Y.config.win).on("resize", Y.throttle(Y.bind(function() {
          this.syncUI();
          this.positionImages()
        }, this), 50), this);
        Y.on("hashchange", Y.bind(this.onHashChange, this), Y.config.win);
        Y.all("#parallax-nav a").each(function(a) {
          a.on("click", function() {
            if (a.getAttribute("href") === window.location.hash) this.onHashChange({
              newHash: a.getAttribute("href").replace("#", "")
            })
          }, this)
        }, this);
        Y.one(".back-to-top-link a").on("click",

        function(a) {
          a.halt();
          this.onHashChange({
            newHash: Y.one("[data-url-id]").getAttribute("data-url-id")
          })
        }, this);
        Y.all("#desktopNav .external-link a[href*=#]").each(function(a) {
          a.on("click", function(b) {
            var e = Y.one(a.getAttribute("href"));
            if (e) {
              var d = e.getXY()[1];
              b.preventDefault();
              this.autoScrolling = !0;
              this.scrollEl.anim({}, {
                to: {
                  scroll: [0, d]
                },
                duration: this.SCROLL_SPEED,
                easing: Y.Easing.easeBoth
              }).run().on("end", function() {
                this.scrollEl.get("scrollTop") !== d && this.scrollEl.set("scrollTop", d);
                this.autoScrolling = !1;
                this.updateActivePage()
              }, this)
            }
          }, this)
        }, this)
      } else Y.one(Y.config.win).on("scroll", Y.bind(function() {
        this.positionBackgroundImage()
      }, this), this), b = Y.UA.mobile ? "orientationchange" : "resize", Y.one(Y.config.win).on(b, Y.throttle(Y.bind(function() {
        this.syncUI();
        this.positionBackgroundImage()
      }, this), 50), this);
      this.setupMobileNav()
    },
    syncUI: function() {
      var b = Y.one("body"),
        a = Y.one(".sqs-announcement-bar"),
        c = Y.one("#header").get("clientHeight"),
        e = 0,
        e = a && a.get("offsetHeight"),
        d = b.hasClass("fixed-header"),
        f = b.hasClass("title--description-position-over-image");
      this.parallaxOff = "false" == Y.Squarespace.Template.getTweakValue("parallax-scrolling");
      this.viewportH = b.get("winHeight");
      this.docHeight = b.get("docHeight");
      if (Y.one("body.collection-type-index")) {
        this.isMobile ? (this.setupMobileLayout(), Y.one("#header").setStyle("position", "absolute"), Y.one(".sqs-cart-dropzone").setStyle("marginTop", c), Y.one(".parallax-images > .image-container:nth-child(1) > img") && Y.one(".title-desc-wrapper").setStyle("minHeight", "600px"), 0 < Y.UA.ie && 9 >= Y.UA.ie ? Y.one(".title-desc-wrapper").setStyle("paddingTop", f ? 80 + c : c) : Y.one(".title-desc-wrapper").setStyle("paddingTop", c)) : (a ? Y.one("#content-wrapper").setStyle("marginTop", d ? c + e : null) : Y.one("#content-wrapper").setStyle("marginTop", d ? c : null), Y.all(".title-desc-wrapper").setStyle("paddingTop", d && f ? 80 + c : null), Y.one(".title-desc-wrapper") && Y.one(".title-desc-wrapper").setStyle("paddingTop", null));
        a = Y.Squarespace.Template.getTweakValue("index-image-height");
        this.IMAGE_VIEWPORT = "Fullscreen" == a ? 1 : "Half" == a ? 0.5 : 0.66;
        this.firstImageHeight = "true" === Y.Squarespace.Template.getTweakValue("first-index-image-fullscreen") ? this.viewportH : parseInt(this.viewportH * this.IMAGE_VIEWPORT);
        this.restImageHeight = parseInt(this.viewportH * this.IMAGE_VIEWPORT);
        var h = new Y.NodeList;
        this.parallaxPages.each(function(a, b) {
          if (!this.isMobile) {
            var g = 0 === b ? this.firstImageHeight - Y.one("#header").get("clientHeight") : this.restImageHeight;
            if (f) {
              var i = d ? c + 160 : 160,
                g = Math.max(g, a.one(".title-desc-inner").height() + i),
                i = 0 === b ? g + c : g;
              this.parallaxImages.item(b).setStyle("height", Math.max(this.viewportH, i) + "px")
            }
            if (i = this.parallaxImages.item(b).one("img")) Y.one(".sqs-announcement-bar") ? a.one(".title-desc-wrapper").setStyle("height", d ? g + "px" : g - e + "px") : a.one(".title-desc-wrapper").setStyle("height", g + "px"), h.push(i.removeAttribute("data-load"))
          }
          this.pageOffsets[a.getAttribute("data-url-id")] = 0 === b ? 0 : Math.round(a.getXY()[1])
        }, this);
        Y.Squarespace.GalleryManager.addImageQueue(h);
        this.parallaxImages.each(function(a) {
          (a = a.one("img")) && a.getAttribute("src") && ImageLoader.load(a)
        }, this);
        this.stickyCart()
      } else(a = Y.one(".banner-image img")) && ImageLoader.load(a), Y.one(".sqs-cart-dropzone").setStyle("marginTop", c + e), this.stickyCart(), this.isMobile || Y.one("#header-wrapper").setStyle("paddingTop", d ? c : null);
      this.isMobile || (Y.one(".collection-type-index.title--description-alignment-center.title--description-position-over-image") && Y.all(".title-desc-wrapper.has-main-image").each(function(a) {
        a.one(".title-desc-inner").setStyles({
          top: "50%",
          left: "50%",
          transform: "translatex(-50%) translatey(-50%)"
        })
      }), Y.one(".collection-type-index.title--description-alignment-left") && Y.all(".title-desc-wrapper.over-image.has-main-image .title-desc-inner").setStyles({
        top: null,
        left: null,
        transform: "translatex(0) translatey(0)"
      }), Y.one("#parallax-nav") && (a = Y.one("#parallax-nav").get("offsetHeight"), Y.one("#parallax-nav").setStyle("marginTop", - 1 * (a / 2))));
      Y.one(".footer-wrapper .sqs-block") || Y.one(".footer-wrapper").addClass("empty");
      Y.one(".nav-wrapper") && b.addClass("has-nav")
    },
    textShrink: function(b, a) {
      Y.one(b) && Y.one(b).ancestor(a) && Y.all(b).each(function(b) {
        b.plug(Y.Squarespace.TextShrink, {
          parentEl: b.ancestor(a)
        })
      })
    },
    setupMobileLayout: function() {
      var b = Y.config.win.innerHeight > Y.config.win.innerWidth ? screen.height : screen.width,
        a = Y.Squarespace.Template.getTweakValue("index-image-height"),
        a = "Two Thirds" == a ? 0.66666 : "Fullscreen" == a ? 1 : 0.5,
        a = a * b;
      Y.all(".parallax-item").each(function(b, e) {
        var d = b.one(".title-desc-wrapper"),
          f = b.one(".title-desc-inner");
        d.hasClass("has-main-image") ? 0 < Y.UA.ie && 9 >= Y.UA.ie ? d.setStyle("height", a) : d.setStyle("minHeight", a) : d.setStyle("paddingTop", Y.one("#header").get("clientHeight"));
        Y.one(".title--description-alignment-center") && d.hasClass("has-main-image") && (f.get("clientHeight") < d.get("clientHeight") && 0 !== e ? f.setStyles({
          position: "absolute",
          top: "50%",
          left: "50%",
          transform: "translatex(-50%) translatey(-50%)"
        }) : f.setStyles({
          position: "relative"
        }), 0 === e && (f.get("clientHeight") + 78 < d.get("clientHeight") - Y.one("#header").height() ? (b.one(".title-desc-inner").setStyle("paddingTop",
        Y.one("#header").get("clientHeight") + 78), f.setStyles({
          position: "absolute",
          top: "50%",
          left: "50%",
          transform: "translatex(-50%) translatey(-50%)"
        })) : f.setStyles({
          position: "relative",
          marginBottom: "78px"
        })))
      });
      !Y.one(".parallax-scrolling") || 0 < Y.UA.ie && 9 >= Y.UA.ie ? (0 < Y.UA.ie && 9 >= Y.UA.ie ? Y.one("body").addClass("crappy-ie-no-parallax") : Y.one("body").addClass("mobile-no-parallax"), Y.all(".title-desc-image").each(function(a, b) {
        0 === b && a.setStyles({
          minHeight: a.ancestor(".title-desc-wrapper").get("clientHeight") + Y.one("#header").get("clientHeight")
        });
        a.one("img").removeAttribute("data-load");
        ImageLoader.load(a.one("img"), {
          mode: "fill"
        })
      })) : (Y.one("body").addClass("mobile-parallax"), Y.all(".title-desc-image").each(function(a, e) {
        0 === e ? a.setStyle("height", a.ancestor(".title-desc-wrapper").get("clientHeight") + Y.one("#header").get("offsetHeight")) : a.setStyle("height", b);
        a.one("img").removeAttribute("data-load");
        ImageLoader.load(a.one("img"), {
          mode: "fill"
        })
      }))
    },
    setupMobileNav: function() {
      Y.one("#mobileMenu").on("click",

      function() {
        Y.one("body").hasClass("mobile-nav-open") ? Y.one("body").removeClass("mobile-nav-open") : Y.one("body").addClass("mobile-nav-open")
      });
      Y.all("li.folder").each(function(b) {
        b.on("click", function() {
          var a = b.siblings("li.folder.dropdown-open").item(0);
          a && a.toggleClass("dropdown-open");
          b && b.toggleClass("dropdown-open")
        })
      })
    },
    positionBackgroundImage: function() {
      var b = this.scrollEl.get("scrollTop"),
        a = Y.one(Y.config.win).get("region"),
        c = Y.one(".banner-image img");
      !this.parallaxOff && (!this.isMobile && c && !(b > a.height)) && c.setStyle("transform", "translate3d(0," + parseInt(b * this.PARALLAX_FACTOR, 10) + "px,0)")
    },
    onHashChange: function(b) {
      Y.one(".mobile-nav-open") && Y.one("body").removeClass("mobile-nav-open");
      if (Y.one('.parallax-item[data-url-id="' + b.newHash + '"]')) {
        var a = this.pageOffsets[b.newHash];
        b.quick ? (this.scrollEl.set("scrollTop", a), this.updateActivePage()) : (this.autoScrolling = !0, this.scrollEl.anim({}, {
          to: {
            scroll: [0, a]
          },
          duration: this.SCROLL_SPEED,
          easing: Y.Easing.easeBoth
        }).run().on("end", function() {
          this.scrollEl.get("scrollTop") !== a && this.scrollEl.set("scrollTop", a);
          this.autoScrolling = !1;
          this.updateActivePage()
        }, this))
      }
    },
    getPageFromOffset: function(b) {
      if (this.parallaxPages.item(0)) {
        var a = this.parallaxPages.item(0).getAttribute("data-url-id"),
          c;
        for (c in this.pageOffsets) b >= this.pageOffsets[c] && this.pageOffsets[c] > this.pageOffsets[a] && (a = c);
        return a
      }
    },
    updateActivePage: function() {
      if (!this.autoScrolling) {
        var b = this.scrollEl.get("scrollTop"),
          a = this.getPageFromOffset(b);
        Y.one("#parallax-nav") && (Y.one('#parallax-nav a[href="#' + a + '"]').get("parentNode").addClass("active").siblings().removeClass("active"), window.location.hash.replace("#", "") != a && window.history && window.history.replaceState && window.history.replaceState({}, "", "#" + a));
        a = this.isMobile ? Y.one('.parallax-item[data-url-id="' + a + '"] .title-desc-wrapper img') : Y.one('#parallax-images .image-container[data-url-id="' + a + '"] img');
        Y.Squarespace.GalleryManager.promoteImageQueue(new Y.NodeList(a));
        if (!Y.one("body.hide-parallax-nav")) {
          var a = this.getPageFromOffset(b + this.viewportH / 2),
            c;
          if (b + this.viewportH / 2 <= this.pageOffsets[a] + (0 === this.pageOffsets[a] ? this.firstImageHeight : this.viewportH * this.IMAGE_VIEWPORT)) c = Y.one('.parallax-item[data-url-id="' + a + '"] .title-desc-wrapper').getAttribute("data-color-suggested");
          if (!c || "#" === c) c = Y.Squarespace.Template.getTweakValue("contentBgColor"), (b = c.match(/rgba\((\d+),(\d+),(\d+),(\d+)/)) && (c = this._rgb2hex(b[1], b[2], b[3]));
          Y.one(".scroll-arrow") && Y.one(".scroll-arrow").removeClass("color-weight-dark").removeClass("color-weight-light").addClass("color-weight-" + this._getLightness(c));
          Y.one("#parallax-nav") && Y.one("#parallax-nav").removeClass("color-weight-dark").removeClass("color-weight-light").addClass("color-weight-" + this._getLightness(c))
        }
      }
    },
    _rgb2hex: function(b, a, c) {
      b = [b, a, c];
      for (a = 0; 2 >= a; ++a) b[a] = parseInt(b[a], 10).toString(16), 1 == b[a].length && (b[a] = "0" + b[a]);
      return "#" + b.join("")
    },
    _getLightness: function(b) {
      return b && 0 < b.length && 7 >= b.length ? (b = b.replace("#", ""), 8388607.5 < parseInt(b, 16) ? "light" : "dark") : ""
    },
    positionImages: function() {
      if (!this.isMobile) {
        var b = this.scrollEl.get("scrollTop"),
          a = Y.one(Y.config.win).get("region");
        this.parallaxPages.each(function(c, e) {
          if (c.inRegion(a)) {
            var d = this.pageOffsets[c.getAttribute("data-url-id")] - b,
              f = -1 * parseInt(d * (this.parallaxOff ? 0 : this.PARALLAX_FACTOR)),
              h = this.parallaxImages.item(e),
              j = h.one("img");
            h.setStyle("transform", "translate3d(0," + d + "px,0)");
            j && j.setStyle("transform", "translate3d(0," + f + "px,0)")
          } else this.parallaxImages.item(e).setStyle("transform", "translate3d(0,-9000px,0)")
        }, this)
      }
    },
    listenTweaks: function() {
      Y.Global && (Y.Global.on("tweak:change", function(b) {
        b.getName().match(/image|parallax|title--description-alignment|fixed-header/i) && this.syncUI()
      }, this), Y.Global.on(["tweak:reset", "tweak:close"], function() {
        Y.later(500, this, this.syncUI)
      }, this))
    },
    initAnnouncementBar: function() {
      var b = Y.one(".sqs-announcement-bar"),
        a = Y.one(".fixed-header");
      if (b && a) {
        var a = b.get("clientHeight"),
          c = Y.one(".sqs-announcement-bar-close"),
          e = Y.one("#header");
        b.setStyles({
          position: "fixed",
          width: "100%"
        });
        e && c && (e.setStyle("top", a), c.on("click",

        function() {
          e.setStyle("top", "0")
        }))
      }
    },
    stickyCart: function() {
      if (this.isMobile) return !1;
      var b = Y.one(".sqs-cart-dropzone");
      Y.one("#header").get("clientHeight");
      var a;
      if (b && b.one(".yui3-widget")) if (a = b.one(".yui3-widget").getY(), Y.one(window).on("resize", function() {
        a = b.getY()
      }), Y.one("body.fixed-header")) b.addClass("fixed-cart").setStyles({
        top: Y.one("#header").get("clientHeight") + 10
      });
      else Y.one(window).on("scroll", function() {
        b.toggleClass("fixed-cart", Y.config.win.scrollY >= a)
      })
    }
  })
});
