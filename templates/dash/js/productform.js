  BuildForm($('#form-product'),[
// product info
      {
        id: 'name',
        name: 'name',
        value: '{{ product.Name }}',
        label: 'Name',
        type: 'text',
        asterisk: false,
        rules: [
          {
            rule: 'required',
            value: true,
            message: 'Enter a name for your product'
          }
        ],
        $parent: $('#product-info')
      },
      {
        id: 'slug',
        name: 'slug',
        value: '{{ product.Slug }}',
        label: 'Slug',
        type: 'text',
        asterisk: false,
        rules: [
          {
            rule: 'required',
            value: true,
            message: 'Enter a unique URL identifier for your product'
          }
        ],
        $parent: $('#product-info')
      },
      {
        id: 'Description',
        name: 'description',
        {% if product.Description %}
        value: {{ jsonify(product.Description) | safe }},
        {% endif %}
        label: 'Description',
        type: 'textarea',
        height: '100px',
        $parent: $('#product-info')
      },
// product cost
      {
        id: 'listPrice',
        name: 'listPrice',
        {% if product.ListPrice %}
        value: Util.renderUICurrencyFromJSON({{ product.ListPrice }}),
        {% else %}
        value: "0.00",
        {% endif %}
        label: 'List Price',
        type: 'currency',
        asterisk: false,
        rules: [
          {
            rule: 'required',
            value: true,
            message: 'Enter a list (suggested retail) price for your product'
          },
          {
            rule: 'check-currency',
            value: ['$,€,£', false],
            message: 'Enter a valid currency $, €, £'
          }
        ],
        $parent: $('#product-cost')
      },
      {
        id: 'price',
        name: 'price',
        {% if product.Price %}
        value: Util.renderUICurrencyFromJSON({{ product.Price }}),
        {% else %}
        value: "0.00",
        {% endif %}
        label: 'Price',
        type: 'currency',
        asterisk: false,
        rules: [
          {
            rule: 'required',
            value: true,
            message: 'Enter a price for your product'
          },
          {
            rule: 'check-currency',
            value: ['$,€,£', false],
            message: 'Enter a valid currency $, €, £'
          }
        ],
        $parent: $('#product-cost')
      },
      {
        id: 'currency',
        name: 'currency',
        label: 'Currency',
        value: [
          {% for type in constants.CurrencyTypes %}
          {
            selected: '{{ type }}' === '{{ product.Currency }}',
            name: '{{ type.Label() }}',
            id: '{{ type }}',
          },
          {% endfor %}
        ],
        type: 'select',
        asterisk: false,
        rules: [
          {
            rule: 'required',
            value: true,
            message: 'Choose a currency for your product'
          }
        ],
        $parent: $('#product-cost')
      },
      {
        id: 'taxable',
        name: 'taxable',
        label: 'Do you wish to collect taxes for this product?',
        type: 'switch',
        value: '{{ product.Taxable }}' === 'True',
        asterisk: false,
        $parent: $('#product-cost')
      },
      {
        id: 'available',
        name: 'available',
        label: 'Is this available for purchase?',
        type: 'switch',
        value: '{{ product.Available }}' === 'True',
        asterisk: false,
        $parent: $('#product-cost')
      },
      {
        id: 'dimensions',
        name: 'dimensions',
        value: '{{ product.Dimensions }}',
        label: 'Size (L x W x H)',
        type: 'text',
        placeholder: '10cm x 10cm x 10cm',
        $parent: $('#product-shipping')
      },
      {
        id: 'weight',
        name: 'weight',
        value: '{{ product.Weight }}',
        label: 'Weight (grams)',
        type: 'text',
        placeholder: '1000',
        $parent: $('#product-shipping'),
        rules: [
          {
            rule: 'number',
            value: true,
            message: 'Weight is not a number'
          }
        ],
      },
  ]);


