csio = window.csio || {};
var templateEl = $('.template');
templateEl.parent().remove();

csio.renderLineItem = function(lineItem, index) {
    var cartEl = templateEl.clone(false);
    cartEl.find('img.thumbnail').attr('src', lineItem.img);
    cartEl.find('input.slug').val(lineItem.slug).attr('name', 'Order.Items.' + (index + 1) + '.Slug');
    cartEl.find('input.sku').val(lineItem.sku).attr('name', 'Order.Items.' + (index + 1) + '.SKU');
    cartEl.find('a.title').text(lineItem.name);
    cartEl.find('div.variant-info').text(lineItem.color + ' / ' + lineItem.size);
    cartEl.find('.quantity input').val(lineItem.quantity).attr('name', 'Order.Items.' + (index + 1) + '.Quantity');
    cartEl.find('.price span').text(lineItem.price);
    cartEl.removeClass('template');

    $('.cart-container tbody').append(cartEl);
}

csio.renderCart = function() {
  var cart = cart || csio.getCart();
  var numItems = 0;
  var subTotal = 0;
  var i = 0;

  for (var k in cart) {
    i += 1;
    var lineItem = cart[k];
    numItems += lineItem.quantity;
    subTotal += lineItem.price * lineItem.quantity;
    csio.renderLineItem(lineItem, i)
  }

  $('.subtotal .price span').text(price);
}

csio.renderCart()
