// Globals
csio = window.csio || {};
csio.product = csio.product || {};

$('.fixed-cart').click(function() {
  window.location = '/cart';
})

if (location.pathname == '/cart') {
  $('.fixed-cart').hide()
}

csio.product.getSKU = function() {
  var SKU = this.Slug;

  $('.variant-option').each(function(i,v) {

  })
}

function checkout() {
    $.cookie.json = true;

    var quantity = parseInt($('#quantity').val(), 10);

    var cart = $.cookie('SkullySystemsCart') || {};

    var cartItemName = $('#color').val() + ' ' + $('#size').val() + ' {{ product.Title }}';

    if(cart[cartItemName]) {
        cart[cartItemName].quantity += quantity;
    } else {
        var newCartItem = {
            img: imgs,
            name: '{{ product.Title }}',
            color: $('#color').val(),
            size: $('#size').val(),
            quantity: quantity,
        }

        cart[cartItemName] = newCartItem;
    }

    $.cookie('SkullySystemsCart', cart, { expires: 7, path: '/' });
}
