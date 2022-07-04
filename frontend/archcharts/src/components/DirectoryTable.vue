<template>

  <div class="grid" ref="grid"></div>

</template>

<script>
import { Grid } from 'gridjs'
export default {
  name: "DirectoryTable",
  mounted() {
    
  },
  data() {
    return {
      columns: ['Make', 'Model', 'Year', 'Color'],
      rows: [
        ['Ford', 'Fusion', '2011', 'Silver'],
        ['Chevrolet', 'Cruz', '2018', 'White']
      ]
    }
  },
  methods: {
    myMethod() {
      console.log("It's ready!")
    }
  }
}
</script>

<style scoped>



</style>