window.csio = window.csio || {};

var $form = $("form#stripeForm");
var $cardNumber = $('input#cardNumber');
var $expiry = $('input#expiry');
var $cvc = $('input#cvcInput');
var $stripeToken = $('input#stripeToken');

// Setting this to true won't help against server-side checks. :)
csio.approved = false;

// For disabling credit card inputs.
// To be used right before form submission.
csio.disable = function ($ele) {
    $ele.disable = true;
};

// Checks each input and does dumb checks to see if it might be a valid card
var validateCard = function() {
    var fail = {
        success: false
    };

    var cardNumber = $cardNumber.val();
    if (cardNumber.length < 10)
        return fail;

    var rawExpiry = $expiry.val().replace(/\s/g, '');
    var arr = rawExpiry.split('/');
    var month = arr[0];
    var year = arr[1];

    if (month.length != 2)
        return fail;
    if (year.length != 2)
        return fail;

    var cvc = $cvc.val();
    if (cvc.length < 3)
        return fail;

    return {
        success: true,
        'number': cardNumber,
        'month': month,
        'year': year,
        'cvc': cvc
    };
};

var authorizeMessage = $('#authorize-message');

// Callback for createToken
var stripeResponseHandler = function(status, response) {
    if (response.error) {
        authorizeMessage.text(response.error.message);
    } else {
        csio.approved = true;
        var token = response.id;
        $stripeToken.val(token);
        authorizeMessage.text('Card approved. Ready when you are.');
    }
};

// Copies validated card values into the hidden form for Stripe.js
function stripeRunner() {
    var card = validateCard();
    if (card.success) {
        $form.find('input[data-stripe="number"]').val(card.number);
        $form.find('input[data-stripe="cvc"]').val(card.cvc);
        $form.find('input[data-stripe="exp-month"]').val(card.month);
        $form.find('input[data-stripe="exp-year"]').val(card.year);

        Stripe.card.createToken($form, stripeResponseHandler);
    }
}

$cardNumber.change(stripeRunner);
$expiry.change(stripeRunner);
$cvc.change(stripeRunner);
