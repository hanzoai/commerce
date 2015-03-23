/*
 *  Document   : app.js
 *  Author     : pixelcave
 *  Description: Custom scripts and plugin initializations
 */

var App = function() {

    /* Initialization UI Code */
    var uiInit = function() {

        // Handle UI
        handleHeader();
        handleMenu();
        scrollToTop();

        // Add the correct copyright year at the footer
        var yearCopy = $('#year-copy'), d = new Date();
        yearCopy.html(d.getFullYear().toString())

        // Initialize tabs
        $('[data-toggle="tabs"] a, .enable-tabs a').click(function(e){ e.preventDefault(); $(this).tab('show'); });

        // Initialize Tooltips
        $('[data-toggle="tooltip"], .enable-tooltip').tooltip({container: 'body', animation: false});

        // Initialize Popovers
        $('[data-toggle="popover"], .enable-popover').popover({container: 'body', animation: true});

        // Initialize Placeholder (for IE9)
        $('input, textarea').placeholder();

        // Initialize Image Lightbox
        $('[data-toggle="lightbox-image"]').magnificPopup({type: 'image', image: {titleSrc: 'title'}});

        // Initialize image gallery lightbox
        $('[data-toggle="lightbox-gallery"]').each(function(){
            $(this).magnificPopup({
                delegate: 'a.gallery-link',
                type: 'image',
                gallery: {
                    enabled: true,
                    navigateByImgClick: true,
                    arrowMarkup: '<button type="button" class="mfp-arrow mfp-arrow-%dir%" title="%title%"></button>',
                    tPrev: 'Previous',
                    tNext: 'Next',
                    tCounter: '<span class="mfp-counter">%curr% of %total%</span>'
                },
                image: {titleSrc: 'title'}
            });
        });

        // Toggle animation class when an element appears with Jquery Appear plugin
        $('[data-toggle="animation-appear"]').each(function(){
            var $this       = $(this);
            var $animClass  = $this.data('animation-class');
            var $elemOff    = $this.data('element-offset');

            $this.appear(function() {
                $this.removeClass('visibility-none').addClass($animClass);
            },{accY: $elemOff});
        });

        // With CountTo (+ help of Jquery Appear plugin), check out examples and documentation at https://github.com/mhuggins/jquery-countTo
        $('[data-toggle="countTo"]').each(function(){
            var $this = $(this);

            $this.appear(function() {
                $this.countTo({
                    speed: 2000,
                    refreshInterval: 20,
                    onComplete: function() {
                        if($this.data('after')) {
                            $this.html($this.html() + $this.data('after'));
                        }
                    }
                });
            });
        });

        // With vPageScroll, check out examples and documentation at https://github.com/nico-martin/vPageScroll (init in IE10 and up)
        if ( ! $('html').hasClass('lt-ie10') ) {
            $('.scroller-container').vpagescroll({
                sectionContainer: '.scroller-container > section',
                sectionInner: '.scroller-container > section > .container',
                navigation: '.scroller-nav'
            });
        }
    };

    /* Handles Main Menu */
    var handleMenu = function(){
        var sideNav = $('.site-nav');

        $('.site-menu-toggle').on('click', function(){
            sideNav.toggleClass('site-nav-visible');
        });

        sideNav.on('mouseleave', function(){
            $(this).removeClass('site-nav-visible');
        });
    };

    /* Scroll to top functionality */
    var scrollToTop = function() {
        // Get link
        var link = $('#to-top');
        var windowW = window.innerWidth
                        || document.documentElement.clientWidth
                        || document.body.clientWidth;

        $(window).scroll(function() {
            // If the user scrolled a bit (150 pixels) show the link in large resolutions
            if (($(this).scrollTop() > 150) && (windowW > 991)) {
                link.fadeIn(100);
            } else {
                link.fadeOut(100);
            }
        });

        // On click get to top
        link.click(function() {
            $('html, body').animate({scrollTop: 0}, 400);
            return false;
        });
    };

    return {
        init: function() {
            uiInit(); // Initialize UI Code
        }
    };
}();

/* Initialize app when page loads */
$(function(){ App.init(); });
