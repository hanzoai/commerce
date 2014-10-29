$('div.field').on('click', function() {
    $(this).removeClass('error');
});

$('#form').submit(function(e) {
    var empty = $('div.required > input').filter(function() {return $(this).val() == "";});
    if (empty.length > 0) {
        e.preventDefault();
        empty.parent().addClass('error');
    }
});

var t = setInterval(function(){
    var empty = $('div:visible.required > input').filter(function() {return $(this).val() == "";});

    if (empty.length == 0) {
        var fieldset = $('div.sqs-checkout-form-payment-content > fieldset');
        fieldset.css('display', 'block');
        fieldset.css('opacity', '0');
        fieldset.fadeTo(1000, 1);
        clearInterval(t);
    }
}, 500);

$('#form').card({
    container: '#card-wrapper',
    numberInput: 'input[name="Order.Account.Number"]',
    expiryInput: 'input[name="RawExpiry"]',
    cvcInput: 'input[name="Order.Account.CVV2"]',
    nameInput: 'input[name="Order.BillingUser.FirstName"], input[name="Order.BillingUser.LastName"]'
});
