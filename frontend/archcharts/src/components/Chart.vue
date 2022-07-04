<template>
  <div ref="chart" class="chart">
  </div>
</template>

<script>
import {GoogleCharts} from 'google-charts';
import {mapState} from 'pinia'
import {useDataStore} from "../stores/data";

export default {
  props: {
    stat: {
      type: String,
      required: true
    }
  },
  computed: {
    ...mapState(useDataStore, ["directories", "allDirectoryStats", "rootDirectory"]),
  },
watch:{
    directories: function(newValue, oldValue){
      this.drawChart();
    }
},
  methods:{

    directorySelected(e){
      console.log(e)
      console.log()
      this.$emit("input", this.directories[e[0].row -1].name);
    },
    toRow(directory){
      return [directory.name, directory.parent, directory[this.stat]]
    },
    drawChart(){
      GoogleCharts.load(this.buildChart, {'packages':['treemap']});
    },
    buildChart() {
      const arrayDataTable = [
        ['Directory', 'Parent', this.stat],
        ["/", null, 0],
        ...this.directories.map(this.toRow)
      ];
      let data = GoogleCharts.api.visualization.arrayToDataTable(arrayDataTable);



      let tree = new GoogleCharts.api.visualization.TreeMap(this.$refs['chart']);

      tree.draw(data, {
        enableHighlight: true,
        showTooltips: true,
        headerHeight: 15,
        fontColor: 'black',
        maxDepth: 1,
      });

      google.visualization.events.addListener(tree, 'select', () =>  this.directorySelected(tree.getSelection()));
    }
  },
  mounted() {
    this.drawChart()
  }
}
</script>

<style scoped>
.chart{
  width: 50vw;
  height: 650px;
}
</style>