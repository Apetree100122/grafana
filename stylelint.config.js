module.exports = { extends: ['stylelint-config-sass-guidelines'],
  ignoreFiles: ['**/node_modules/**/*.scss'],
  rules: {'at-rule-no-vendor-prefix': null,'color-hex-length': null,
    'color-named': null,'declaration-block-no-duplicate-properties': 
      [ true, {ignore: 'consecutive-duplicates-with-different-values',                                                               
               ignoreProperties: ['font-size', 'word-break']}                                                            
      ],//   Disable equivalent "borderZero" sass-lint rule
    'declaration-property-value-disallowed-list': {  border: [],
      'border-bottom': [],  'border-left': [],
      'border-right': [], 'border-top': [],    
                                                  }, 'function-url-quotes': null,'length-zero-no-unit': null,'max-nesting-depth': null,
    'property-no-vendor-prefix': null, 'rule-empty-line-before': null,'scss/at-function-pattern': null,'scss/at-mixin-pattern': null,
    'scss/dollar-variable-pattern': null,'scss/at-extend-no-missing-placeholder': null,'selector-class-pattern': null,'selector-max-compound-selectors': null, 'selector-max-id': null,
    'selector-no-qualifying-type': null, 'selector-pseudo-element-colon-notation': null, 'shorthand-property-no-redundant-values': null,'value-no-vendor-prefix': null},
};
