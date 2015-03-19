/*
 *  Document   : submitProfile.js
 *  Author     : verusmedia
 *  Description: Custom javascript code used on Profile page
 */

var SubmitPassword = function() {

    return {
        init: function() {
            /*
             *  Jquery Validation, Check out more examples and documentation at https://github.com/jzaefferer/jquery-validation
             */

            /* Login form - Initialize Validation */
            $('#form-password').validate({
                errorClass: 'help-block animation-slideUp', // You can change the animation class for a different entrance animation - check animations page
                errorElement: 'div',
                errorPlacement: function(error, e) {
                    e.parents('.form-group > div').append(error);
                },
                highlight: function(e) {
                    $(e).closest('.form-group').removeClass('has-success has-error').addClass('has-error');
                    $(e).closest('.help-block').remove();
                },
                success: function(e) {
                    e.closest('.form-group').removeClass('has-success has-error');
                    e.closest('.help-block').remove();
                },
                rules: {
                    'OldPassword': {
                        required: true,
                        minlength: 6
                    },
                    'Password': {
                        required: true,
                        minlength: 6
                    },
                    'ConfirmPassword': {
                        required: true,
                        minlength: 6,
                        equalTo: "#password"
                    },
                },
                messages: {
                    'OldPassword': {
                        required: 'Please provide your password',
                        minlength: 'Your password must be at least 6 characters long'
                    },
                    'Password': {
                        required: 'Please provide your password',
                        minlength: 'Your password must be at least 6 characters long'
                    },
                    'ConfirmPassword': {
                        required: 'Please provide your password',
                        minlength: 'Your password must be at least 6 characters long',
                        equalTo: 'You did not type the same password twice'
                    }
                }
            });
        }
    };
}();

